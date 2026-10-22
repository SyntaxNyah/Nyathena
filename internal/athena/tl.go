package athena

// TimeLeap (TL) — a JSON-only, server->client custom packet carrying a full room
// snapshot so a client can rewind its view.
//
// TL is a SECOND-CLASS extension, exactly like GP: it is registered through
// aolib's custom-packet entry point, it is not in spec/, and it has no
// conformance vector. Unlike GP it requires BOTH gates before it is emitted —
// JSON mode AND a client->server FL advertising "timeleap" — so a JSON peer that
// has no TL handler never has its room state rewritten (see supportsTimeLeap).

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
	// tlFeature is the FL capability name. Lowercase: Nyathena's SupportsFeature
	// is a case-sensitive map lookup, unlike AsyncAO's FeatureSet.
	tlFeature = "timeleap"

	// tlDefaultTTL is how long a snapshot stays restorable.
	tlDefaultTTL = 15 * time.Minute

	// Restore modes.
	//
	//   authoritative - the room itself is rolled back (background/music are
	//                   broadcast to everyone) and TL carries the transcript to
	//                   TL-capable peers. Every peer agrees on the scene.
	//   overlay       - nothing server-side changes; only TL-capable peers
	//                   rewind. This deliberately shows them an older scene.
	tlModeAuthoritative = "authoritative"
	tlModeOverlay       = "overlay"
)

// TLCastMember is one participant's render state at capture time.
type TLCastMember struct {
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

// TLLogLine is one replayed IC message, oldest first.
type TLLogLine struct {
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

// TLMusic mirrors the MC fields needed to re-stage the track.
type TLMusic struct {
	Name     string `json:"name"`
	CharID   int    `json:"char_id"`
	Showname string `json:"showname"`
	Looping  bool   `json:"looping"`
}

// TLArea is the room-level half of a snapshot.
type TLArea struct {
	Name       string   `json:"name"`
	Background string   `json:"background"`
	Side       string   `json:"side"`
	Music      TLMusic  `json:"music"`
	Status     int      `json:"status"`
	Evidence   []string `json:"evidence"`
	Testimony  []string `json:"testimony"`
}

// TLState is the restorable scene.
type TLState struct {
	Area TLArea         `json:"area"`
	Cast []TLCastMember `json:"cast"`
	Log  []TLLogLine    `json:"log"`
}

// TLPacket is the wire form. Op distinguishes the broadcast restore from the
// private save ack; Mode is the restore mode described above.
type TLPacket struct {
	Op         string  `json:"op"`
	SnapshotID string  `json:"snapshot_id"`
	Label      string  `json:"label"`
	Divergence string  `json:"divergence"`
	IssuedBy   int     `json:"issued_by"`
	IssuedAt   int64   `json:"issued_at"`
	TTLSeconds int     `json:"ttl_s"`
	Mode       string  `json:"mode"`
	State      TLState `json:"state"`
}

// Header returns the wire header "TL".
func (p *TLPacket) Header() string { return "TL" }

// Args is unused: TL is JSON-only and never encoded as FantaCode.
func (p *TLPacket) Args() []string { return nil }

// tlCodec registers TL's JSON-only outbound form. GP parity: there is no
// FantaCode form because TL only ever goes to JSON peers. Unlike gpCodec the
// Fanta closures are non-nil and return an error, so a direct
// packetutil.Encode(..., WireFanta) is a logged drop instead of a nil-func panic.
func tlCodec() packetutil.Codec {
	errJSONOnly := errors.New("athena: TL is JSON-only")
	return packetutil.Codec{
		EncodeFanta: func(any) ([]string, error) { return nil, errJSONOnly },
		EncodeJSON: func(p any) ([]byte, error) {
			return json.Marshal(p.(*TLPacket)) // $header is added by ensureHeader
		},
	}
}

// supportsTimeLeap reports whether this client may receive TL frames.
//
// BOTH guards are required, and neither is redundant:
//   - JSONMode(): TL is JSON-only. Send() bypasses packetutil.Encode for
//     FantaCode clients, so without this guard a Fanta peer would be sent a
//     meaningless empty "TL#%" frame.
//   - SupportsFeature(): the peer must have advertised "timeleap" in a
//     client->server FL, so a JSON client with no TL handler never has its room
//     rewritten.
//
// This is deliberately NOT supportsGroupPair()'s shortcut (pairgroup.go), which
// returns jsonMode alone and therefore pushes GP to every JSON peer regardless
// of what it advertised.
func (client *Client) supportsTimeLeap() bool {
	return client.JSONMode() && client.SupportsFeature(tlFeature)
}

// --- snapshot store ---------------------------------------------------------

// tlEntry is one stored snapshot plus its expiry (zero = never expires).
type tlEntry struct {
	pkt     *TLPacket
	expires time.Time
}

var tlStore = struct {
	mu  sync.Mutex
	m   map[*area.Area]map[string]*tlEntry
	seq map[*area.Area]int
}{
	m:   make(map[*area.Area]map[string]*tlEntry),
	seq: make(map[*area.Area]int),
}

// nextSnapshotID mints "wr-N" per area. Ids are never reused, so a stale id
// cannot silently address a newer snapshot after a TTL sweep.
func nextSnapshotID(a *area.Area) string {
	tlStore.mu.Lock()
	defer tlStore.mu.Unlock()
	tlStore.seq[a]++
	return "wr-" + strconv.Itoa(tlStore.seq[a])
}

func storeTimeLeap(a *area.Area, p *TLPacket) {
	tlStore.mu.Lock()
	defer tlStore.mu.Unlock()
	if tlStore.m[a] == nil {
		tlStore.m[a] = make(map[string]*tlEntry)
	}
	var exp time.Time
	if p.TTLSeconds > 0 {
		exp = time.Now().Add(time.Duration(p.TTLSeconds) * time.Second)
	}
	tlStore.m[a][p.SnapshotID] = &tlEntry{pkt: p, expires: exp}
}

// lookupTimeLeap resolves an id, expiring lazily: no sweeper goroutine, so a
// snapshot costs nothing until someone asks for it.
func lookupTimeLeap(a *area.Area, id string) (*TLPacket, bool) {
	tlStore.mu.Lock()
	defer tlStore.mu.Unlock()
	byID := tlStore.m[a]
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

func listTimeLeaps(a *area.Area) []string {
	tlStore.mu.Lock()
	defer tlStore.mu.Unlock()
	now := time.Now()
	out := make([]string, 0, len(tlStore.m[a]))
	for id, e := range tlStore.m[a] {
		if !e.expires.IsZero() && now.After(e.expires) {
			continue
		}
		out = append(out, id+" ("+e.pkt.Label+")")
	}
	sort.Strings(out)
	return out
}

// --- capture / restore ------------------------------------------------------

// tlDivergence is presentation-only (the meter's readout). D3 — encoding real
// state in the value — is deliberately deferred.
const tlDivergence = "1.048596"

// captureTimeLeap snapshots the area the issuer is in.
func captureTimeLeap(a *area.Area, label, mode string, issuer *Client) *TLPacket {
	p := &TLPacket{
		Op:         "restore",
		Label:      label,
		Mode:       mode,
		Divergence: tlDivergence,
		SnapshotID: nextSnapshotID(a),
		IssuedBy:   issuer.Uid(),
		IssuedAt:   time.Now().Unix(),
		TTLSeconds: int(tlDefaultTTL / time.Second),
	}
	p.State.Area = TLArea{
		Name:       a.Name(),
		Background: a.Background(),
		Side:       issuer.Pos(),
		Music:      TLMusic{Name: a.CurrentSong(), CharID: -1, Looping: true},
		Status:     int(a.Status()),
		Evidence:   append([]string(nil), a.Evidence()...),
		Testimony:  append([]string(nil), a.Testimony()...),
	}

	clients.ForEach(func(c *Client) {
		if c.Area() != a || c.CharID() < 0 {
			return
		}
		pi := c.PairInfo()
		p.State.Cast = append(p.State.Cast, TLCastMember{
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

	for _, l := range tlLogFor(a).snapshot() {
		p.State.Log = append(p.State.Log, TLLogLine{
			TS: l.TS, UID: l.UID, CharID: l.CharID, Name: l.Name, Emote: l.Emote,
			Showname: l.Showname, Side: l.Side, Text: l.Text, Color: l.Color,
		})
	}
	return p
}

// applyTimeLeapRoom rolls the area itself back (authoritative mode). Only the
// room-level half is applied here: background and music are what every peer
// renders without the transcript, and both have a first-class packet. The
// transcript reaches TL-capable peers through TL itself.
func applyTimeLeapRoom(a *area.Area, p *TLPacket) {
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

// restoreTimeLeap pushes the snapshot to TL-gated clients only and reports how
// many received it.
func restoreTimeLeap(a *area.Area, p *TLPacket) int {
	sent := 0
	clients.ForEach(func(c *Client) {
		if c.Area() != a || !c.supportsTimeLeap() {
			return
		}
		c.Send(p) // JSON path only: the gate above guarantees it
		sent++
	})
	return sent
}

// --- command ----------------------------------------------------------------

// registerTimeLeapCommand runs after initCommands (server.go), which is the
// documented contract for RegisterCommand.
func registerTimeLeapCommand() {
	RegisterCommand("tl", Command{
		handler:    cmdTimeLeap,
		minArgs:    1,
		usage:      "Usage: /tl save <name> | /tl <id> [authoritative|overlay] | /tl list",
		desc:       "Saves or restores a room snapshot (TimeLeap).",
		reqPerms:   permissions.PermissionField["NONE"],
		category:   "general",
		publicHelp: true,
	})
}

func cmdTimeLeap(client *Client, args []string, _ string) {
	switch args[0] {
	case "save":
		if len(args) < 2 {
			client.SendServerMessage("Usage: /tl save <name>")
			return
		}
		label := strings.Join(args[1:], " ")
		p := captureTimeLeap(client.Area(), label, tlModeAuthoritative, client)
		storeTimeLeap(client.Area(), p)

		// Private ack: same worldline id, log omitted so saving does not
		// re-render the issuer's own room. Gated on BOTH guards like every other
		// TL emission: a JSON peer that never advertised the feature has no TL
		// handler, and Send would otherwise hand a FantaCode peer an empty frame.
		if client.supportsTimeLeap() {
			ack := *p
			ack.Op = "save"
			ack.State.Log = nil
			client.Send(&ack)
		}
		client.SendServerMessage(fmt.Sprintf("Saved worldline %s (%s).", p.SnapshotID, label))
		addToBuffer(client, "CMD", "Saved TL snapshot "+p.SnapshotID+".", false)

	case "list":
		ids := listTimeLeaps(client.Area())
		if len(ids) == 0 {
			client.SendServerMessage("No worldlines saved in this area.")
			return
		}
		client.SendServerMessage("Worldlines: " + strings.Join(ids, ", "))

	default:
		mode := tlModeAuthoritative
		if len(args) > 1 {
			switch args[1] {
			case tlModeAuthoritative, tlModeOverlay:
				mode = args[1]
			default:
				client.SendServerMessage("Usage: /tl <id> [authoritative|overlay]")
				return
			}
		}
		stored, ok := lookupTimeLeap(client.Area(), args[0])
		if !ok {
			client.SendServerMessage("No such worldline: " + args[0])
			return
		}
		out := *stored
		out.Op = "restore"
		out.Mode = mode

		if mode == tlModeAuthoritative {
			applyTimeLeapRoom(client.Area(), &out)
		}
		n := restoreTimeLeap(client.Area(), &out)

		client.SendServerMessage(fmt.Sprintf(
			"Restored worldline %s (%s) — %d client(s) rewound.", stored.SnapshotID, stored.Label, n))
		addToBuffer(client, "CMD", "Restored TL snapshot "+stored.SnapshotID+" ("+mode+").", false)
	}
}
