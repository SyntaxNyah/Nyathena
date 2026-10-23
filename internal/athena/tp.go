package athena

// Teleport (TP) — a JSON-only, server->client custom packet carrying a full room
// snapshot so a client can rewind its view.
//
// TP is a SECOND-CLASS extension, exactly like GP: it is registered through
// aolib's custom-packet entry point, it is not in spec/, and it has no
// conformance vector. Unlike GP it requires BOTH gates before it is emitted —
// JSON mode AND a client->server FL advertising "teleport" — so a JSON peer that
// has no TP handler never has its room state rewritten (see supportsTeleport).

import (
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	aolib "github.com/AO-Underground/aolib/go/v2"

	"github.com/MangosArentLiterature/Athena/internal/area"
	"github.com/MangosArentLiterature/Athena/internal/packetutil"
	"github.com/MangosArentLiterature/Athena/internal/permissions"
)

const (
	// tpFeature is the FL capability name. Lowercase: Nyathena's SupportsFeature
	// is a case-sensitive map lookup, unlike AsyncAO's FeatureSet.
	tpFeature = "teleport"

	// tpDefaultTTL is how long a snapshot stays restorable.
	tpDefaultTTL = 15 * time.Minute

	// Restore modes.
	//
	//   authoritative - the room itself is rolled back (background/music are
	//                   broadcast to everyone) and TP carries the transcript to
	//                   TP-capable peers. Every peer agrees on the scene.
	//   overlay       - nothing server-side changes; only TP-capable peers
	//                   rewind. This deliberately shows them an older scene.
	tpModeAuthoritative = "authoritative"
	tpModeOverlay       = "overlay"
)

// TPCastMember is one participant's render state at capture time.
type TPCastMember struct {
	UID      int          `json:"uid"`
	CharID   int          `json:"char_id"`
	Name     string       `json:"name"`
	Emote    string       `json:"emote"`
	Showname string       `json:"showname"`
	Side     string       `json:"side"`
	Offset   aolib.Offset `json:"offset"`
	Flip     aolib.Flip   `json:"flip"`
	Order    int          `json:"order"`
}

// TPLogLine is one replayed IC message, oldest first.
type TPLogLine struct {
	TS       int64  `json:"ts"`
	UID      int    `json:"uid"`
	CharID   int    `json:"char_id"`
	Name     string `json:"name"`
	Emote    string `json:"emote"`
	Showname string `json:"showname"`
	Side     string `json:"side"`
	Text     string `json:"text"`
	Color    int    `json:"color"`
}

// TPMusic mirrors the MC fields needed to re-stage the track.
type TPMusic struct {
	Name     string `json:"name"`
	CharID   int    `json:"char_id"`
	Showname string `json:"showname"`
	Looping  bool   `json:"looping"`
}

// TPArea is the room-level half of a snapshot.
type TPArea struct {
	Name       string   `json:"name"`
	Background string   `json:"background"`
	Side       string   `json:"side"`
	Music      TPMusic  `json:"music"`
	Status     int      `json:"status"`
	Evidence   []string `json:"evidence"`
	Testimony  []string `json:"testimony"`
}

// TPState is the restorable scene.
type TPState struct {
	Area TPArea         `json:"area"`
	Cast []TPCastMember `json:"cast"`
	Log  []TPLogLine    `json:"log"`
}

// TPPacket is the wire form. Op distinguishes the broadcast restore from the
// private save ack; Mode is the restore mode described above.
type TPPacket struct {
	Op         string  `json:"op"`
	SnapshotID string  `json:"snapshot_id"`
	Label      string  `json:"label"`
	Divergence string  `json:"divergence"`
	IssuedBy   int     `json:"issued_by"`
	IssuedAt   int64   `json:"issued_at"`
	TTLSeconds int     `json:"ttl_s"`
	Mode       string  `json:"mode"`
	State      TPState `json:"state"`
}

// Header returns the wire header "TP".
func (p *TPPacket) Header() string { return "TP" }

// Args is unused: TP is JSON-only and never encoded as FantaCode.
func (p *TPPacket) Args() []string { return nil }

// tpCodec registers TP's JSON-only outbound form. GP parity: there is no
// FantaCode form because TP only ever goes to JSON peers. Unlike gpCodec the
// Fanta closures are non-nil and return an error, so a direct
// packetutil.Encode(..., WireFanta) is a logged drop instead of a nil-func panic.
func tpCodec() packetutil.Codec {
	errJSONOnly := errors.New("athena: TP is JSON-only")
	return packetutil.Codec{
		EncodeFanta: func(any) ([]string, error) { return nil, errJSONOnly },
		EncodeJSON: func(p any) ([]byte, error) {
			return json.Marshal(p.(*TPPacket)) // $header is added by ensureHeader
		},
	}
}

// supportsTeleport reports whether this client may receive TP frames.
//
// BOTH guards are required, and neither is redundant:
//   - JSONMode(): TP is JSON-only. Send() bypasses packetutil.Encode for
//     FantaCode clients, so without this guard a Fanta peer would be sent a
//     meaningless empty "TP#%" frame.
//   - SupportsFeature(): the peer must have advertised "teleport" in a
//     client->server FL, so a JSON client with no TP handler never has its room
//     rewritten.
//
// This is deliberately NOT supportsGroupPair()'s shortcut (pairgroup.go), which
// returns jsonMode alone and therefore pushes GP to every JSON peer regardless
// of what it advertised.
func (client *Client) supportsTeleport() bool {
	return client.JSONMode() && client.SupportsFeature(tpFeature)
}

// --- snapshot store ---------------------------------------------------------

// tpEntry is one stored snapshot plus its expiry (zero = never expires).
type tpEntry struct {
	pkt     *TPPacket
	expires time.Time
}

var tpStore = struct {
	mu  sync.Mutex
	m   map[*area.Area]map[string]*tpEntry
	seq map[*area.Area]int
}{
	m:   make(map[*area.Area]map[string]*tpEntry),
	seq: make(map[*area.Area]int),
}

// nextSnapshotID mints "wr-N" per area. Ids are never reused, so a stale id
// cannot silently address a newer snapshot after a TTL sweep.
func nextSnapshotID(a *area.Area) string {
	tpStore.mu.Lock()
	defer tpStore.mu.Unlock()
	tpStore.seq[a]++
	return "wr-" + strconv.Itoa(tpStore.seq[a])
}

func storeTeleport(a *area.Area, p *TPPacket) {
	tpStore.mu.Lock()
	defer tpStore.mu.Unlock()
	if tpStore.m[a] == nil {
		tpStore.m[a] = make(map[string]*tpEntry)
	}
	var exp time.Time
	if p.TTLSeconds > 0 {
		exp = time.Now().Add(time.Duration(p.TTLSeconds) * time.Second)
	}
	tpStore.m[a][p.SnapshotID] = &tpEntry{pkt: p, expires: exp}
}

// lookupTeleport resolves an id, expiring lazily: no sweeper goroutine, so a
// snapshot costs nothing until someone asks for it.
func lookupTeleport(a *area.Area, id string) (*TPPacket, bool) {
	tpStore.mu.Lock()
	defer tpStore.mu.Unlock()
	byID := tpStore.m[a]
	e, ok := byID[id]
	if !ok {
		return nil, false
	}
	if !e.expires.IsZero() && time.Now().After(e.expires) {
		delete(byID, id)
		return nil, false
	}
	return e.pkt, true
}

func listTeleports(a *area.Area) []string {
	tpStore.mu.Lock()
	defer tpStore.mu.Unlock()
	now := time.Now()
	out := make([]string, 0, len(tpStore.m[a]))
	for id, e := range tpStore.m[a] {
		if !e.expires.IsZero() && now.After(e.expires) {
			continue
		}
		out = append(out, id+" ("+e.pkt.Label+")")
	}
	sort.Strings(out)
	return out
}

// --- capture / restore ------------------------------------------------------

// tpDivergence is presentation-only (the meter's readout). D3 — encoding real
// state in the value — is deliberately deferred.
const tpDivergence = "1.048596"

// captureTeleport snapshots the area the issuer is in.
func captureTeleport(a *area.Area, label, mode string, issuer *Client) *TPPacket {
	p := &TPPacket{
		Op:         "restore",
		Label:      label,
		Mode:       mode,
		Divergence: tpDivergence,
		SnapshotID: nextSnapshotID(a),
		IssuedBy:   issuer.Uid(),
		IssuedAt:   time.Now().Unix(),
		TTLSeconds: int(tpDefaultTTL / time.Second),
	}
	p.State.Area = TPArea{
		Name:       a.Name(),
		Background: a.Background(),
		Side:       issuer.Pos(),
		Music:      TPMusic{Name: a.CurrentSong(), CharID: -1, Looping: true},
		Status:     int(a.Status()),
		Evidence:   append([]string(nil), a.Evidence()...),
		Testimony:  append([]string(nil), a.Testimony()...),
	}

	clients.ForEach(func(c *Client) {
		if c.Area() != a || c.CharID() < 0 {
			return
		}
		pi := c.PairInfo()
		p.State.Cast = append(p.State.Cast, TPCastMember{
			UID:      c.Uid(),
			CharID:   c.CharID(),
			Name:     pi.name,
			Emote:    pi.emote,
			Showname: c.Showname(),
			Side:     c.Pos(),
			Offset:   parseOffset(pi.offset),
			Flip:     parsePairFlip(pi.flip),
		})
	})
	// Deterministic z-order: by char id, then renumbered so the Order values
	// stay contiguous even if a member left mid-capture.
	sort.SliceStable(p.State.Cast, func(i, j int) bool {
		return p.State.Cast[i].CharID < p.State.Cast[j].CharID
	})
	for i := range p.State.Cast {
		p.State.Cast[i].Order = i
	}

	for _, l := range tpLogFor(a).snapshot() {
		p.State.Log = append(p.State.Log, TPLogLine{
			TS: l.TS, UID: l.UID, CharID: l.CharID, Name: l.Name, Emote: l.Emote,
			Showname: l.Showname, Side: l.Side, Text: l.Text, Color: l.Color,
		})
	}
	return p
}

// applyTeleportRoom rolls the area itself back (authoritative mode). Only the
// room-level half is applied here: background and music are what every peer
// renders without the transcript, and both have a first-class packet. The
// transcript reaches TP-capable peers through TP itself.
func applyTeleportRoom(a *area.Area, p *TPPacket) {
	if bg := p.State.Area.Background; bg != "" {
		a.SetBackground(bg)
		broadcastToArea(a, &aolib.BN{Background: bg, Position: p.State.Area.Side})
	}
	if name := p.State.Area.Music.Name; name != "" {
		a.SetCurrentSong(name)
		broadcastToArea(a, &aolib.MCToClient{
			Name:     name,
			CharID:   p.State.Area.Music.CharID,
			Showname: p.State.Area.Music.Showname,
			Looping:  p.State.Area.Music.Looping,
			Channel:  aolib.MusicChannelMusic,
			Effects:  aolib.MusicEffects{},
		})
	}
}

// restoreTeleport pushes the snapshot to TP-gated clients only and reports how
// many received it.
func restoreTeleport(a *area.Area, p *TPPacket) int {
	sent := 0
	clients.ForEach(func(c *Client) {
		if c.Area() != a || !c.supportsTeleport() {
			return
		}
		c.Send(p) // JSON path only: the gate above guarantees it
		sent++
	})
	return sent
}

// --- command ----------------------------------------------------------------

// registerTeleportCommand runs after initCommands (server.go), which is the
// documented contract for RegisterCommand.
func registerTeleportCommand() {
	RegisterCommand("tp", Command{
		handler:    cmdTP,
		minArgs:    1,
		usage:      "Usage: /tp save <name> | /tp <id> [authoritative|overlay] | /tp list",
		desc:       "Saves or restores a room snapshot (Teleport).",
		reqPerms:   permissions.PermissionField["NONE"],
		category:   "general",
		publicHelp: true,
	})
}

func cmdTP(client *Client, args []string, _ string) {
	switch args[0] {
	case "save":
		if len(args) < 2 {
			client.SendServerMessage("Usage: /tp save <name>")
			return
		}
		label := strings.Join(args[1:], " ")
		p := captureTeleport(client.Area(), label, tpModeAuthoritative, client)
		storeTeleport(client.Area(), p)

		// Private ack: same worldline id, log omitted so saving does not
		// re-render the issuer's own room. Gated on BOTH guards like every other
		// TP emission: a JSON peer that never advertised the feature has no TP
		// handler, and Send would otherwise hand a FantaCode peer an empty frame.
		if client.supportsTeleport() {
			ack := *p
			ack.Op = "save"
			ack.State.Log = nil
			client.Send(&ack)
		}
		client.SendServerMessage(fmt.Sprintf("Saved worldline %s (%s).", p.SnapshotID, label))
		addToBuffer(client, "CMD", "Saved TP snapshot "+p.SnapshotID+".", false)

	case "list":
		ids := listTeleports(client.Area())
		if len(ids) == 0 {
			client.SendServerMessage("No worldlines saved in this area.")
			return
		}
		client.SendServerMessage("Worldlines: " + strings.Join(ids, ", "))

	default:
		mode := tpModeAuthoritative
		if len(args) > 1 {
			switch args[1] {
			case tpModeAuthoritative, tpModeOverlay:
				mode = args[1]
			default:
				client.SendServerMessage("Usage: /tp <id> [authoritative|overlay]")
				return
			}
		}
		stored, ok := lookupTeleport(client.Area(), args[0])
		if !ok {
			client.SendServerMessage("No such worldline: " + args[0])
			return
		}
		out := *stored
		out.Op = "restore"
		out.Mode = mode

		if mode == tpModeAuthoritative {
			applyTeleportRoom(client.Area(), &out)
		}
		n := restoreTeleport(client.Area(), &out)

		client.SendServerMessage(fmt.Sprintf(
			"Restored worldline %s (%s) — %d client(s) rewound.", stored.SnapshotID, stored.Label, n))
		addToBuffer(client, "CMD", "Restored TP snapshot "+stored.SnapshotID+" ("+mode+").", false)
	}
}
