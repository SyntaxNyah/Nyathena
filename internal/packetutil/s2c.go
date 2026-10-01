package packetutil

import (
	"encoding/json"

	aolib "github.com/AO-Underground/aolib/go/v2"
)

// s2cParsers re-exposes aolib's server→client decoders for Nyathena's
// type-erased SendPacket(header, args...) escape hatch. aolib keeps its own
// s2c registry private (and only drives it through the session API), so this
// thin map lets Nyathena reconstruct a typed packet from positional args and
// re-encode it to correct JSON.
var s2cParsers = map[string]func([]string) (aolib.Outgoing, error){
	"ARUP":       func(b []string) (aolib.Outgoing, error) { return aolib.ParseARUP(b) },
	"ASS":        func(b []string) (aolib.Outgoing, error) { return aolib.ParseASS(b) },
	"AUTH":       func(b []string) (aolib.Outgoing, error) { return aolib.ParseAUTH(b) },
	"BB":         func(b []string) (aolib.Outgoing, error) { return aolib.ParseBB(b) },
	"BD":         func(b []string) (aolib.Outgoing, error) { return aolib.ParseBD(b) },
	"BN":         func(b []string) (aolib.Outgoing, error) { return aolib.ParseBN(b) },
	"CHECK":      func(b []string) (aolib.Outgoing, error) { return aolib.ParseCHECK(b) },
	"CT":         func(b []string) (aolib.Outgoing, error) { return aolib.ParseCTToClient(b) },
	"CharsCheck": func(b []string) (aolib.Outgoing, error) { return aolib.ParseCharsCheck(b) },
	"decryptor":  func(b []string) (aolib.Outgoing, error) { return aolib.ParseDecryptor(b) },
	"DONE":       func(b []string) (aolib.Outgoing, error) { return aolib.ParseDONE(b) },
	"EE":         func(b []string) (aolib.Outgoing, error) { return aolib.ParseEE(b) },
	"EM":         func(b []string) (aolib.Outgoing, error) { return aolib.ParseEM(b) },
	"FA":         func(b []string) (aolib.Outgoing, error) { return aolib.ParseFA(b) },
	"FM":         func(b []string) (aolib.Outgoing, error) { return aolib.ParseFM(b) },
	"HP":         func(b []string) (aolib.Outgoing, error) { return aolib.ParseHPToClient(b) },
	"ID":         func(b []string) (aolib.Outgoing, error) { return aolib.ParseIDToClient(b) },
	"JD":         func(b []string) (aolib.Outgoing, error) { return aolib.ParseJD(b) },
	"KB":         func(b []string) (aolib.Outgoing, error) { return aolib.ParseKB(b) },
	"KK":         func(b []string) (aolib.Outgoing, error) { return aolib.ParseKK(b) },
	"LE":         func(b []string) (aolib.Outgoing, error) { return aolib.ParseLE(b) },
	"MA":         func(b []string) (aolib.Outgoing, error) { return aolib.ParseMA(b) },
	"MC":         func(b []string) (aolib.Outgoing, error) { return aolib.ParseMCToClient(b) },
	"MS":         func(b []string) (aolib.Outgoing, error) { return aolib.ParseMSToClient(b) },
	"PN":         func(b []string) (aolib.Outgoing, error) { return aolib.ParsePN(b) },
	"PR":         func(b []string) (aolib.Outgoing, error) { return aolib.ParsePR(b) },
	"PU":         func(b []string) (aolib.Outgoing, error) { return aolib.ParsePU(b) },
	"PV":         func(b []string) (aolib.Outgoing, error) { return aolib.ParsePV(b) },
	"RT":         func(b []string) (aolib.Outgoing, error) { return aolib.ParseRTToClient(b) },
	"SC":         func(b []string) (aolib.Outgoing, error) { return aolib.ParseSC(b) },
	"SI":         func(b []string) (aolib.Outgoing, error) { return aolib.ParseSI(b) },
	"SM":         func(b []string) (aolib.Outgoing, error) { return aolib.ParseSM(b) },
	"SP":         func(b []string) (aolib.Outgoing, error) { return aolib.ParseSP(b) },
	"TI":         func(b []string) (aolib.Outgoing, error) { return aolib.ParseTI(b) },
	"ZZ":         func(b []string) (aolib.Outgoing, error) { return aolib.ParseZZToClient(b) },
}

// BuildJSONFromArgs encodes a type-erased (header, positional-args) packet to
// its correct JSON wire form. It reconstructs the typed packet from the args
// (custom codecs through their DecodeFanta, canonical packets through the s2c
// parsers) and lets aolib's typed encoder produce the JSON.
func BuildJSONFromArgs(header string, args []string) ([]byte, error) {
	if c, ok := customCodecs[header]; ok {
		p, err := c.DecodeFanta(args)
		if err != nil {
			return nil, err
		}
		return encodeCustom(header, p, c, aolib.WireJSON)
	}
	if parse, ok := s2cParsers[header]; ok {
		p, err := parse(args)
		if err != nil {
			return nil, err
		}
		return aolib.Encode(p, aolib.WireJSON)
	}
	// Unknown/empty packet: emit the bare envelope.
	obj := map[string]json.RawMessage{}
	h, _ := json.Marshal(header)
	obj["$header"] = h
	return json.Marshal(obj)
}
