package keygen

import (
	"math/big"

	"github.com/intel-go/fastjson"
	"github.com/torusresearch/torus-public/common"
	"github.com/torusresearch/torus-public/logging"
	fjson "github.com/valyala/fastjson"
)

// Dont try to cast, use the exported structs to get the types you need
type pssPhase string
type pssDealer bool
type pssPlayer bool
type pssReceivedSend bool

type pssPhases struct {
	Initial   pssPhase
	Started   pssPhase
	Proposing pssPhase
	Ended     pssPhase
}

type pssDealers struct {
	IsDealer  pssDealer
	NotDealer pssDealer
}

type pssPlayers struct {
	IsPlayer  pssPlayer
	NotPlayer pssPlayer
}

type pssReceivedSends struct {
	True  pssReceivedSend
	False pssReceivedSend
}

type PSSState struct {
	Phase        pssPhase
	Dealer       pssDealer
	Player       pssPlayer
	ReceivedSend pssReceivedSend
}

var PSSTypes = struct {
	Phases       pssPhases
	Dealer       pssDealers
	Player       pssPlayers
	ReceivedSend pssReceivedSends
}{
	pssPhases{
		Initial:   pssPhase("initial"),
		Started:   pssPhase("started"),
		Proposing: pssPhase("proposing"),
		Ended:     pssPhase("ended"),
	},
	pssDealers{
		IsDealer:  pssDealer(true),
		NotDealer: pssDealer(false),
	},
	pssPlayers{
		IsPlayer:  pssPlayer(true),
		NotPlayer: pssPlayer(false),
	},
	pssReceivedSends{
		True:  pssReceivedSend(true),
		False: pssReceivedSend(false),
	},
}

type PSSMsgShare struct {
	SharingID SharingID
}

func (pssMsgShare *PSSMsgShare) FromBytes(byt []byte) error {
	var p fjson.Parser
	val, err := p.Parse(string(byt))
	if err != nil {
		return err
	}
	pssMsgShare.SharingID = SharingID(val.GetStringBytes("sharingid"))
	return nil
}

func (pssMsgShare *PSSMsgShare) ToBytes() []byte {
	byt, err := fastjson.Marshal(pssMsgShare)
	if err != nil {
		logging.Error("Could not marshal PSSMsgShare struct")
		return []byte("")
	}
	return byt
}

type PSSMsgRecover struct {
	SharingID SharingID
}

func (pssMsgRecover *PSSMsgRecover) FromBytes(byt []byte) error {
	var p fjson.Parser
	val, err := p.Parse(string(byt))
	if err != nil {
		return err
	}
	pssMsgRecover.SharingID = SharingID(val.GetStringBytes("sharingid"))
	return nil
}

func (pssMsgRecover *PSSMsgRecover) ToBytes() []byte {
	byt, err := fastjson.Marshal(pssMsgRecover)
	if err != nil {
		logging.Error("Could not mpsshal PSSMsgRecover struct")
		return []byte("")
	}
	return byt
}

type Parser interface {
	MarshalI(big.Int) string
	MarshalIA([]big.Int) []string
	MarshalIM([][]big.Int) [][]string
	UnmarshalI(string) big.Int
	UnmarshalIA([]string) []big.Int
	UnmarshalIM([][]string) [][]big.Int
	MarshalP(common.Point) [2]string
	MarshalPA([]common.Point) [][2]string
	MarshalPM([][]common.Point) [][][2]string
	UnmarshalP([2]string) common.Point
	UnmarshalPA([][2]string) []common.Point
	UnmarshalPM([][][2]string) [][]common.Point
}

var BaseParser = &parser{}

type parser struct {
}

func (p *parser) MarshalI(arg big.Int) string {
	return arg.Text(16)
}
func (p *parser) UnmarshalI(arg string) (res big.Int) {
	res.SetString(arg, 16)
	return
}
func (p *parser) MarshalIA(args []big.Int) (res []string) {
	for _, i := range args {
		res = append(res, p.MarshalI(i))
	}
	return
}
func (p *parser) UnmarshalIA(args []string) (res []big.Int) {
	for _, i := range args {
		res = append(res, p.UnmarshalI(i))
	}
	return
}
func (p *parser) MarshalIM(args [][]big.Int) (res [][]string) {
	for _, i := range args {
		res = append(res, p.MarshalIA(i))
	}
	return
}
func (p *parser) UnmarshalIM(args [][]string) (res [][]big.Int) {
	for _, i := range args {
		res = append(res, p.UnmarshalIA(i))
	}
	return
}
func (p *parser) MarshalP(arg common.Point) [2]string {
	return [2]string{arg.X.Text(16), arg.Y.Text(16)}
}
func (p *parser) UnmarshalP(arg [2]string) common.Point {
	var x, y big.Int
	x.SetString(arg[0], 16)
	y.SetString(arg[1], 16)
	return common.Point{X: x, Y: y}
}
func (p *parser) MarshalPA(args []common.Point) (res [][2]string) {
	for _, i := range args {
		res = append(res, p.MarshalP(i))
	}
	return
}
func (p *parser) UnmarshalPA(args [][2]string) (res []common.Point) {
	for _, i := range args {
		res = append(res, p.UnmarshalP(i))
	}
	return
}
func (p *parser) MarshalPM(args [][]common.Point) (res [][][2]string) {
	for _, i := range args {
		res = append(res, p.MarshalPA(i))
	}
	return
}
func (p *parser) UnmarshalPM(args [][][2]string) (res [][]common.Point) {
	for _, i := range args {
		res = append(res, p.UnmarshalPA(i))
	}
	return
}

type PSSMsgSend struct {
	PSSID  PSSID
	C      [][]common.Point
	A      []big.Int
	Aprime []big.Int
	B      []big.Int
	Bprime []big.Int
}

type pssMsgSendBase struct {
	PSSID  string
	C      [][][2]string
	A      []string
	Aprime []string
	B      []string
	Bprime []string
}

func (pssMsgSend *PSSMsgSend) ToBytes() []byte {
	byt, _ := fastjson.Marshal(pssMsgSendBase{
		string(pssMsgSend.PSSID),
		BaseParser.MarshalPM(pssMsgSend.C),
		BaseParser.MarshalIA(pssMsgSend.A),
		BaseParser.MarshalIA(pssMsgSend.Aprime),
		BaseParser.MarshalIA(pssMsgSend.B),
		BaseParser.MarshalIA(pssMsgSend.Bprime),
	})
	return byt
}

func (pssMsgSend *PSSMsgSend) FromBytes(data []byte) error {
	var p pssMsgSendBase
	err := fastjson.Unmarshal(data, &p)
	if err != nil {
		return err
	}
	pssMsgSend.PSSID = PSSID(p.PSSID)
	pssMsgSend.C = BaseParser.UnmarshalPM(p.C)
	pssMsgSend.A = BaseParser.UnmarshalIA(p.A)
	pssMsgSend.Aprime = BaseParser.UnmarshalIA(p.Aprime)
	pssMsgSend.B = BaseParser.UnmarshalIA(p.B)
	pssMsgSend.Bprime = BaseParser.UnmarshalIA(p.Bprime)
	return nil
}

type PSSMsgEcho struct {
	PSSID      PSSID
	C          [][]common.Point
	Alpha      big.Int
	Alphaprime big.Int
	Beta       big.Int
	Betaprime  big.Int
}

type pssMsgEchoBase struct {
	PSSID      string
	C          [][][2]string
	Alpha      string
	Alphaprime string
	Beta       string
	Betaprime  string
}

func (pssMsgEcho *PSSMsgEcho) ToBytes() []byte {
	byt, _ := fastjson.Marshal(pssMsgEchoBase{
		PSSID:      string(pssMsgEcho.PSSID),
		C:          BaseParser.MarshalPM(pssMsgEcho.C),
		Alpha:      BaseParser.MarshalI(pssMsgEcho.Alpha),
		Alphaprime: BaseParser.MarshalI(pssMsgEcho.Alphaprime),
		Beta:       BaseParser.MarshalI(pssMsgEcho.Beta),
		Betaprime:  BaseParser.MarshalI(pssMsgEcho.Betaprime),
	})
	return byt
}

func (pssMsgEcho *PSSMsgEcho) FromBytes(data []byte) error {
	var p pssMsgEchoBase
	err := fastjson.Unmarshal(data, &p)
	if err != nil {
		return err
	}
	pssMsgEcho.PSSID = PSSID(p.PSSID)
	pssMsgEcho.C = BaseParser.UnmarshalPM(p.C)
	pssMsgEcho.Alpha = BaseParser.UnmarshalI(p.Alpha)
	pssMsgEcho.Alphaprime = BaseParser.UnmarshalI(p.Alphaprime)
	pssMsgEcho.Beta = BaseParser.UnmarshalI(p.Beta)
	pssMsgEcho.Betaprime = BaseParser.UnmarshalI(p.Betaprime)
	return nil
}
