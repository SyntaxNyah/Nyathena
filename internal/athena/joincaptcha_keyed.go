package athena

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/binary"
	"strings"
	"sync"
	"time"
)

// Keyed challenge derivation.
//
// crypto/rand alone already makes an individual challenge unpredictable, so a
// key buys nothing there. What it buys is a property a CSPRNG cannot give:
// *determinism the attacker cannot compute*.
//
// Challenges are derived as HMAC-SHA256(serverKey, ipid || bucket) and the
// resulting digest drives every random choice the generators make. Two things
// follow:
//
//   - A connection cannot reroll. Without this, a bot whose solver only handles
//     arithmetic would simply disconnect and reconnect until it drew an
//     arithmetic question -- turning a fifteen-kind pool into a one-kind pool at
//     the cost of a few TCP handshakes. Keyed by IPID, every reconnect inside
//     the rotation window returns the *same* challenge, so there is nothing to
//     fish for. This is the main reason the key exists.
//
//   - A solver cannot be written from the source alone. The generators are
//     public, but which one a given IPID meets, and with what parameters, is a
//     function of a key that is not in this repository. HMAC-SHA256 is a
//     one-way PRF: observing any number of issued challenges reveals nothing
//     about the key, and therefore nothing about what any other address will be
//     asked. Two servers running identical code produce unrelated challenge
//     streams.
//
// The key comes from secretseed in config.toml, domain-separated so it cannot
// collide with the lockdown passkey MAC that shares that seed. With secretseed
// unset the server generates a random key at startup instead: the anti-reroll
// property still holds for the life of the process, it just does not survive a
// restart.

// joinCaptchaKeyDomain separates this key from any other use of secretseed.
// Never reuse a raw key for two purposes -- a signature under one scheme must
// never be a valid derivation under another.
const joinCaptchaKeyDomain = "nyathena-join-captcha-v1"

var (
	joinCaptchaKeyOnce sync.Once
	joinCaptchaKeyVal  []byte
)

// joinCaptchaKey returns the derived per-server challenge key.
func joinCaptchaKey() []byte {
	joinCaptchaKeyOnce.Do(func() {
		var seed string
		if config != nil {
			seed = strings.TrimSpace(config.JoinCaptchaSecret)
			if seed == "" {
				seed = strings.TrimSpace(config.SecretSeed)
			}
		}
		if seed != "" {
			mac := hmac.New(sha256.New, []byte(seed))
			mac.Write([]byte(joinCaptchaKeyDomain))
			joinCaptchaKeyVal = mac.Sum(nil)
			return
		}
		// No configured seed: a random key for this process. Challenges stay
		// unpredictable and unrerollable; they simply differ after a restart.
		k := make([]byte, 32)
		if _, err := rand.Read(k); err != nil {
			// A failing system CSPRNG is not something to paper over with a
			// constant -- fall back to a time-derived key so the server still
			// starts, and let the (extremely unlikely) weakness be local to
			// this process rather than baked into the binary.
			binary.BigEndian.PutUint64(k, uint64(time.Now().UnixNano()))
		}
		joinCaptchaKeyVal = k
	})
	return joinCaptchaKeyVal
}

// joinCaptchaRotateSeconds is how long one IPID keeps the same challenge.
// Long enough that a reconnect loop cannot fish for an easier question, short
// enough that a challenge does not become a fixture. 0 disables keying: each
// connection then draws an independent challenge from crypto/rand.
func joinCaptchaRotateSeconds() int64 {
	if config == nil || config.JoinCaptchaRotate == 0 {
		return 3600
	}
	if config.JoinCaptchaRotate < 0 {
		return 0
	}
	return int64(config.JoinCaptchaRotate)
}

// captchaRand is a deterministic random source driven by an HMAC-SHA256 stream
// in counter mode, or by crypto/rand when unkeyed. Generators take one of these
// rather than calling a package-level function, so the same generator code
// produces a keyed or an unkeyed challenge depending only on how it was seeded.
type captchaRand struct {
	key   []byte
	seed  []byte
	block [sha256.Size]byte
	pos   int // read offset into block; len(block) means "exhausted"
	ctr   uint64
	keyed bool
}

// newKeyedRand returns a source whose output is fully determined by the server
// key and the given label -- the same label always yields the same challenge.
func newKeyedRand(label string) *captchaRand {
	r := &captchaRand{key: joinCaptchaKey(), seed: []byte(label), keyed: true}
	r.pos = len(r.block)
	return r
}

// newUnkeyedRand returns a source backed by crypto/rand, used when keying is
// switched off.
func newUnkeyedRand() *captchaRand {
	r := &captchaRand{}
	r.pos = len(r.block)
	return r
}

// challengeRandFor returns the source used to build a challenge for an IPID.
// Within a rotation window the same IPID always maps to the same stream, which
// is what removes the reconnect-and-reroll attack.
func challengeRandFor(ipid string) *captchaRand {
	rotate := joinCaptchaRotateSeconds()
	if rotate <= 0 {
		return newUnkeyedRand()
	}
	bucket := time.Now().Unix() / rotate
	var b [8]byte
	binary.BigEndian.PutUint64(b[:], uint64(bucket))
	return newKeyedRand(ipid + "\x00" + string(b[:]))
}

// refill advances the HMAC counter and loads the next block of stream bytes.
func (r *captchaRand) refill() {
	if !r.keyed {
		if _, err := rand.Read(r.block[:]); err != nil {
			// Degrade to the keyed path rather than returning a fixed block.
			mac := hmac.New(sha256.New, joinCaptchaKey())
			var b [8]byte
			binary.BigEndian.PutUint64(b[:], r.ctr)
			mac.Write(b[:])
			copy(r.block[:], mac.Sum(nil))
		}
		r.ctr++
		r.pos = 0
		return
	}
	mac := hmac.New(sha256.New, r.key)
	mac.Write(r.seed)
	var b [8]byte
	binary.BigEndian.PutUint64(b[:], r.ctr)
	mac.Write(b[:])
	copy(r.block[:], mac.Sum(nil))
	r.ctr++
	r.pos = 0
}

// nextByte returns the next byte of the stream.
func (r *captchaRand) nextByte() byte {
	if r.pos >= len(r.block) {
		r.refill()
	}
	b := r.block[r.pos]
	r.pos++
	return b
}

// intn returns a uniform value in [0,n).
//
// Rejection sampling, not modulo: taking a 32-bit value mod n skews the low
// values whenever n does not divide 2^32, which over a whole challenge stream
// would bias which kinds and which words appear. Cheap to do correctly, so it
// is done correctly.
func (r *captchaRand) intn(n int) int {
	if n <= 0 {
		return 0
	}
	if n == 1 {
		return 0
	}
	limit := ^uint32(0) - (^uint32(0) % uint32(n)) - 1
	for {
		v := uint32(r.nextByte())<<24 | uint32(r.nextByte())<<16 |
			uint32(r.nextByte())<<8 | uint32(r.nextByte())
		if v <= limit {
			return int(v % uint32(n))
		}
	}
}
