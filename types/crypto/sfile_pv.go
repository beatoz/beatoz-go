package crypto

import (
	"bytes"
	"fmt"
	"github.com/beatoz/beatoz-go/libs"
	"github.com/beatoz/beatoz-go/libs/jsonx"
	"github.com/beatoz/beatoz-go/types"
	abytes "github.com/beatoz/beatoz-go/types/bytes"
	"github.com/beatoz/beatoz-go/types/xerrors"
	"github.com/tendermint/tendermint/crypto/secp256k1"
	tmtypes "github.com/tendermint/tendermint/types"
	"os"
	"time"

	"github.com/gogo/protobuf/proto"

	"github.com/tendermint/tendermint/crypto"
	tmos "github.com/tendermint/tendermint/libs/os"
	"github.com/tendermint/tendermint/libs/protoio"
	"github.com/tendermint/tendermint/libs/tempfile"
	tmproto "github.com/tendermint/tendermint/proto/tendermint/types"
	tmtime "github.com/tendermint/tendermint/types/time"
)

const (
	stepNone      int8 = 0 // Used to distinguish the initial state
	stepPropose   int8 = 1
	stepPrevote   int8 = 2
	stepPrecommit int8 = 3
)

// A vote is either stepPrevote or stepPrecommit.
func voteToStep(vote *tmproto.Vote) int8 {
	switch vote.Type {
	case tmproto.PrevoteType:
		return stepPrevote
	case tmproto.PrecommitType:
		return stepPrecommit
	default:
		panic(fmt.Sprintf("Unknown vote type: %v", vote.Type))
	}
}

//-------------------------------------------------------------------------------

// SFilePVKey stores the immutable part of PrivValidator.
type SFilePVKey struct {
	Address types.Address  `json:"address"`
	PubKey  crypto.PubKey  `json:"pub_key"`
	PrivKey crypto.PrivKey `json:"priv_key"`

	filePath string
}

// Save persists the SFilePVKey to its filePath.
//func (pvKey SFilePVKey) Save() {
//	outFile := pvKey.filePath
//	if outFile == "" {
//		panic("cannot save PrivValidator key: filePath not set")
//	}
//
//	jsonBytes, err := jsonx.MarshalIndent(pvKey, "", "  ")
//	if err != nil {
//		panic(err)
//	}
//
//	err = tempfile.WriteFileAtomic(outFile, jsonBytes, 0600)
//	if err != nil {
//		panic(err)
//	}
//}

// Save persists the SFilePVKey to its filePath.
func (pvKey SFilePVKey) SaveWith(s []byte) {
	outFile := pvKey.filePath
	if outFile == "" {
		panic("cannot save PrivValidator key: filePath not set")
	}

	// build WalletKey contains jsonBytes
	walKey := NewWalletKeyWith(pvKey.PrivKey.Bytes(), s)
	_, err := walKey.Save(libs.NewFileWriter(outFile))
	if err != nil {
		panic(err)
	}

	//jsonWalKey, err := jsonx.MarshalIndent(walKey, "", "  ")
	//if err != nil {
	//	panic(err)
	//}

	//err = tempfile.WriteFileAtomic(outFile, jsonWalKey, 0600)
	//if err != nil {
	//	panic(err)
	//}
}

//-------------------------------------------------------------------------------

// SFilePVLastSignState stores the mutable part of PrivValidator.
type SFilePVLastSignState struct {
	Height    int64           `json:"height"`
	Round     int32           `json:"round"`
	Step      int8            `json:"step"`
	Signature []byte          `json:"signature,omitempty"`
	SignBytes abytes.HexBytes `json:"signbytes,omitempty"`

	filePath string
}

// CheckHRS checks the given height, round, step (HRS) against that of the
// SFilePVLastSignState. It returns an error if the arguments constitute a regression,
// or if they match but the SignBytes are empty.
// The returned boolean indicates whether the last Signature should be reused -
// it returns true if the HRS matches the arguments and the SignBytes are not empty (indicating
// we have already signed for this HRS, and can reuse the existing signature).
// It panics if the HRS matches the arguments, there's a SignBytes, but no Signature.
func (lss *SFilePVLastSignState) CheckHRS(height int64, round int32, step int8) (bool, error) {

	if lss.Height > height {
		return false, xerrors.From(fmt.Errorf("height regression. Got %v, last height %v", height, lss.Height))
	}

	if lss.Height == height {
		if lss.Round > round {
			return false, xerrors.From(fmt.Errorf("round regression at height %v. Got %v, last round %v", height, round, lss.Round))
		}

		if lss.Round == round {
			if lss.Step > step {
				return false, xerrors.From(fmt.Errorf(
					"step regression at height %v round %v. Got %v, last step %v",
					height,
					round,
					step,
					lss.Step,
				))
			} else if lss.Step == step {
				if lss.SignBytes != nil {
					if lss.Signature == nil {
						panic("pv: Signature is nil but SignBytes is not!")
					}
					return true, nil
				}
				return false, xerrors.NewOrdinary("no SignBytes found")
			}
		}
	}
	return false, nil
}

// Save persists the FilePvLastSignState to its filePath.
func (lss *SFilePVLastSignState) Save() {
	outFile := lss.filePath
	if outFile == "" {
		panic("cannot save SFilePVLastSignState: filePath not set")
	}
	jsonBytes, err := jsonx.MarshalIndent(lss, "", "  ")
	if err != nil {
		panic(err)
	}
	err = tempfile.WriteFileAtomic(outFile, jsonBytes, 0600)
	if err != nil {
		panic(err)
	}
}

//-------------------------------------------------------------------------------

// SFilePV implements PrivValidator using data persisted to disk
// to prevent double signing.
// NOTE: the directories containing pv.Key.filePath and pv.LastSignState.filePath must already exist.
// It includes the LastSignature and LastSignBytes so we don't lose the signature
// if the process crashes after signing but before the resulting consensus message is processed.
type SFilePV struct {
	Key           SFilePVKey
	LastSignState SFilePVLastSignState
}

// NewSFilePV generates a new validator from the given key and paths.
func NewSFilePV(privKey crypto.PrivKey, keyFilePath, stateFilePath string) *SFilePV {
	return &SFilePV{
		Key: SFilePVKey{
			Address:  abytes.HexBytes(privKey.PubKey().Address()),
			PubKey:   privKey.PubKey(),
			PrivKey:  privKey,
			filePath: keyFilePath,
		},
		LastSignState: SFilePVLastSignState{
			Step:     stepNone,
			filePath: stateFilePath,
		},
	}
}

// GenSFilePV generates a new validator with randomly generated private key
// and sets the filePaths, but does not call Save().
func GenSFilePV(keyFilePath, stateFilePath string) *SFilePV {
	return NewSFilePV(secp256k1.GenPrivKey(), keyFilePath, stateFilePath)
}

// LoadSFilePV loads a SFilePV from the filePaths.  The SFilePV handles double
// signing prevention by persisting data to the stateFilePath.  If either file path
// does not exist, the program will exit.
func LoadSFilePV(keyFilePath, stateFilePath string, s []byte) *SFilePV {
	return loadSFilePV(keyFilePath, stateFilePath, true, s)
}

// LoadSFilePVEmptyState loads a SFilePV from the given keyFilePath, with an empty LastSignState.
// If the keyFilePath does not exist, the program will exit.
func LoadSFilePVEmptyState(keyFilePath, stateFilePath string, s []byte) *SFilePV {
	return loadSFilePV(keyFilePath, stateFilePath, false, s)
}

// If loadState is true, we load from the stateFilePath. Otherwise, we use an empty LastSignState.
func loadSFilePV(keyFilePath, stateFilePath string, loadState bool, s []byte) *SFilePV {
	jsonWalKeyBytes, err := os.ReadFile(keyFilePath)
	if err != nil {
		tmos.Exit(err.Error())
	}

	walKey := WalletKey{}
	err = jsonx.Unmarshal(jsonWalKeyBytes, &walKey)
	if err != nil {
		tmos.Exit(fmt.Sprintf("Error reading PrivValidator key from %v: %v\n", keyFilePath, err))
	}

	err = walKey.Unlock(s)
	if err != nil {
		tmos.Exit(err.Error())
	}

	keyBytes := walKey.PrvKey()
	if err != nil {
		panic(err) //tmos.Exit(err.Error())
	}
	cloneKeyBytes := make([]byte, len(keyBytes))
	copy(cloneKeyBytes, keyBytes)
	walKey.Lock()

	pvKey := SFilePVKey{
		PrivKey: secp256k1.PrivKey(cloneKeyBytes),
	}

	// overwrite pubkey and address for convenience
	pvKey.PubKey = pvKey.PrivKey.PubKey()
	pvKey.Address = abytes.HexBytes(pvKey.PubKey.Address())
	pvKey.filePath = keyFilePath

	pvState := SFilePVLastSignState{}

	if loadState {
		stateJSONBytes, err := os.ReadFile(stateFilePath)
		if err != nil {
			tmos.Exit(err.Error())
		}
		err = jsonx.Unmarshal(stateJSONBytes, &pvState)
		if err != nil {
			tmos.Exit(fmt.Sprintf("Error reading PrivValidator state from %v: %v\n", stateFilePath, err))
		}
	}

	pvState.filePath = stateFilePath

	return &SFilePV{
		Key:           pvKey,
		LastSignState: pvState,
	}
}

// LoadOrGenSFilePV loads a SFilePV from the given filePaths
// or else generates a new one and saves it to the filePaths.
func LoadOrGenSFilePV(keyFilePath, stateFilePath string, s []byte) *SFilePV {
	var pv *SFilePV
	if tmos.FileExists(keyFilePath) {
		pv = LoadSFilePV(keyFilePath, stateFilePath, s)
		// retry to save.
		// each time the file is saved, the salt value will be changed,
		// so the file content will be changed also.
		pv.SaveWith(s)
	} else {
		pv = GenSFilePV(keyFilePath, stateFilePath)
		pv.SaveWith(s)
	}
	return pv
}

// GetAddress returns the address of the validator.
// Implements PrivValidator.
func (pv *SFilePV) GetAddress() types.Address {
	return pv.Key.Address
}

// GetPubKey returns the public key of the validator.
// Implements PrivValidator.
func (pv *SFilePV) GetPubKey() (crypto.PubKey, error) {
	return pv.Key.PubKey, nil
}

// SignVote signs a canonical representation of the vote, along with the
// chainID. Implements PrivValidator.
func (pv *SFilePV) SignVote(chainID string, vote *tmproto.Vote) error {
	if err := pv.signVote(chainID, vote); err != nil {
		return xerrors.From(fmt.Errorf("error signing vote: %v", err))
	}
	return nil
}

// SignProposal signs a canonical representation of the proposal, along with
// the chainID. Implements PrivValidator.
func (pv *SFilePV) SignProposal(chainID string, proposal *tmproto.Proposal) error {
	if err := pv.signProposal(chainID, proposal); err != nil {
		return xerrors.From(fmt.Errorf("error signing proposal: %v", err))
	}
	return nil
}

// Save persists the SFilePV to disk.
//func (pv *SFilePV) Save() {
//	pv.Key.Save()
//	pv.LastSignState.Save()
//}

func (pv *SFilePV) SaveWith(s []byte) {
	pv.Key.SaveWith(s)
	pv.LastSignState.Save()
}

// Reset resets all fields in the SFilePV.
// NOTE: Unsafe!
//func (pv *SFilePV) Reset() {
//	var sig []byte
//	pv.LastSignState.Height = 0
//	pv.LastSignState.Round = 0
//	pv.LastSignState.Step = 0
//	pv.LastSignState.Signature = sig
//	pv.LastSignState.SignBytes = nil
//	pv.Save()
//}

func (pv *SFilePV) ResetWith(s []byte) {
	var sig []byte
	pv.LastSignState.Height = 0
	pv.LastSignState.Round = 0
	pv.LastSignState.Step = 0
	pv.LastSignState.Signature = sig
	pv.LastSignState.SignBytes = nil
	pv.SaveWith(s)
}

// String returns a string representation of the SFilePV.
func (pv *SFilePV) String() string {
	return fmt.Sprintf(
		"PrivValidator{%v LH:%v, LR:%v, LS:%v}",
		pv.GetAddress(),
		pv.LastSignState.Height,
		pv.LastSignState.Round,
		pv.LastSignState.Step,
	)
}

//------------------------------------------------------------------------------------

// signVote checks if the vote is good to sign and sets the vote signature.
// It may need to set the timestamp as well if the vote is otherwise the same as
// a previously signed vote (ie. we crashed after signing but before the vote hit the WAL).
func (pv *SFilePV) signVote(chainID string, vote *tmproto.Vote) error {
	height, round, step := vote.Height, vote.Round, voteToStep(vote)

	lss := pv.LastSignState

	sameHRS, err := lss.CheckHRS(height, round, step)
	if err != nil {
		return err
	}

	signBytes := tmtypes.VoteSignBytes(chainID, vote)

	// We might crash before writing to the wal,
	// causing us to try to re-sign for the same HRS.
	// If signbytes are the same, use the last signature.
	// If they only differ by timestamp, use last timestamp and signature
	// Otherwise, return error
	if sameHRS {
		if bytes.Equal(signBytes, lss.SignBytes) {
			vote.Signature = lss.Signature
		} else if timestamp, ok := checkVotesOnlyDifferByTimestamp(lss.SignBytes, signBytes); ok {
			vote.Timestamp = timestamp
			vote.Signature = lss.Signature
		} else {
			err = xerrors.From(fmt.Errorf("conflicting data"))
		}
		return err
	}

	// It passed the checks. Sign the vote
	sig, err := pv.Key.PrivKey.Sign(signBytes)
	if err != nil {
		return err
	}
	pv.saveSigned(height, round, step, signBytes, sig)
	vote.Signature = sig
	return nil
}

// signProposal checks if the proposal is good to sign and sets the proposal signature.
// It may need to set the timestamp as well if the proposal is otherwise the same as
// a previously signed proposal ie. we crashed after signing but before the proposal hit the WAL).
func (pv *SFilePV) signProposal(chainID string, proposal *tmproto.Proposal) error {
	height, round, step := proposal.Height, proposal.Round, stepPropose

	lss := pv.LastSignState

	sameHRS, err := lss.CheckHRS(height, round, step)
	if err != nil {
		return err
	}

	signBytes := tmtypes.ProposalSignBytes(chainID, proposal)

	// We might crash before writing to the wal,
	// causing us to try to re-sign for the same HRS.
	// If signbytes are the same, use the last signature.
	// If they only differ by timestamp, use last timestamp and signature
	// Otherwise, return error
	if sameHRS {
		if bytes.Equal(signBytes, lss.SignBytes) {
			proposal.Signature = lss.Signature
		} else if timestamp, ok := checkProposalsOnlyDifferByTimestamp(lss.SignBytes, signBytes); ok {
			proposal.Timestamp = timestamp
			proposal.Signature = lss.Signature
		} else {
			err = xerrors.From(fmt.Errorf("conflicting data"))
		}
		return err
	}

	// It passed the checks. Sign the proposal
	sig, err := pv.Key.PrivKey.Sign(signBytes)
	if err != nil {
		return err
	}
	pv.saveSigned(height, round, step, signBytes, sig)
	proposal.Signature = sig
	return nil
}

// Persist height/round/step and signature
func (pv *SFilePV) saveSigned(height int64, round int32, step int8,
	signBytes []byte, sig []byte) {

	pv.LastSignState.Height = height
	pv.LastSignState.Round = round
	pv.LastSignState.Step = step
	pv.LastSignState.Signature = sig
	pv.LastSignState.SignBytes = signBytes
	pv.LastSignState.Save()
}

//-----------------------------------------------------------------------------------------

// returns the timestamp from the lastSignBytes.
// returns true if the only difference in the votes is their timestamp.
func checkVotesOnlyDifferByTimestamp(lastSignBytes, newSignBytes []byte) (time.Time, bool) {
	var lastVote, newVote tmproto.CanonicalVote
	if err := protoio.UnmarshalDelimited(lastSignBytes, &lastVote); err != nil {
		panic(fmt.Sprintf("LastSignBytes cannot be unmarshalled into vote: %v", err))
	}
	if err := protoio.UnmarshalDelimited(newSignBytes, &newVote); err != nil {
		panic(fmt.Sprintf("signBytes cannot be unmarshalled into vote: %v", err))
	}

	lastTime := lastVote.Timestamp
	// set the times to the same value and check equality
	now := tmtime.Now()
	lastVote.Timestamp = now
	newVote.Timestamp = now

	return lastTime, proto.Equal(&newVote, &lastVote)
}

// returns the timestamp from the lastSignBytes.
// returns true if the only difference in the proposals is their timestamp
func checkProposalsOnlyDifferByTimestamp(lastSignBytes, newSignBytes []byte) (time.Time, bool) {
	var lastProposal, newProposal tmproto.CanonicalProposal
	if err := protoio.UnmarshalDelimited(lastSignBytes, &lastProposal); err != nil {
		panic(fmt.Sprintf("LastSignBytes cannot be unmarshalled into proposal: %v", err))
	}
	if err := protoio.UnmarshalDelimited(newSignBytes, &newProposal); err != nil {
		panic(fmt.Sprintf("signBytes cannot be unmarshalled into proposal: %v", err))
	}

	lastTime := lastProposal.Timestamp
	// set the times to the same value and check equality
	now := tmtime.Now()
	lastProposal.Timestamp = now
	newProposal.Timestamp = now

	return lastTime, proto.Equal(&newProposal, &lastProposal)
}
