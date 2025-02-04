package stake

import (
	"github.com/beatoz/beatoz-go/libs"
	"github.com/beatoz/beatoz-go/types/bytes"
	"github.com/beatoz/beatoz-go/types/crypto"
	"github.com/ethereum/go-ethereum/common/math"
	"github.com/stretchr/testify/require"
	"github.com/tendermint/tendermint/crypto/secp256k1"
	"github.com/tendermint/tendermint/libs/rand"
	"sort"
	"testing"
)

func TestStakeCtrler_validatorUpdates(t *testing.T) {
	for c := 1; c < 100; c++ {
		testValidatorUpdates(t, c, 21)
	}
}

func testValidatorUpdates(t *testing.T, delegateeCnt, maxValCnt int) {
	//
	// create delegatees
	var allDelegatees DelegateeArray
	maxCapPower := math.MaxInt64 / int64(delegateeCnt)
	for i := 0; i < delegateeCnt; i++ {
		pubBytes := secp256k1.GenPrivKey().PubKey().Bytes()
		addr, _ := crypto.PubBytes2Addr(pubBytes)
		d := NewDelegatee(addr, pubBytes)
		d.TotalPower = rand.Int63n(maxCapPower)
		allDelegatees = append(allDelegatees, d)
	}

	//
	// check order by power
	sort.Sort(PowerOrderDelegatees(allDelegatees))
	prePower := int64(0)
	for i, d := range allDelegatees {
		if i == 0 {
			prePower = d.TotalPower
		} else {
			require.True(t, prePower >= d.TotalPower)
		}
	}

	//
	// select validator set when previous validator set is nil
	n := libs.MinInt(maxValCnt, len(allDelegatees))
	newVals1 := selectValidators(PowerOrderDelegatees(allDelegatees), n)
	require.NotNil(t, newVals1)

	sort.Sort(AddressOrderDelegatees(newVals1))
	valUps := validatorUpdates(nil, newVals1)

	// Because the previous validator set is nil,
	// the new validator set should be equal to the updated validator set.
	var lastValidators DelegateeArray
	for _, v := range valUps {
		pubBytes := v.PubKey.GetSecp256K1()
		addr, _ := crypto.PubBytes2Addr(pubBytes)
		d := NewDelegatee(addr, pubBytes)
		d.TotalPower = v.Power
		lastValidators = append(lastValidators, d)
	}
	require.Equal(t, newVals1, lastValidators)
	for i, _ := range newVals1 {
		require.Equal(t, newVals1[i].Addr, lastValidators[i].Addr)
		require.Equal(t, newVals1[i].TotalPower, lastValidators[i].TotalPower)
	}

	//
	// update delegatees
	var retAllDelegatees DelegateeArray

	for _, d := range allDelegatees {
		choice := rand.Int() % 4
		if delegateeCnt <= 10 && choice == 1 {
			choice = 0 // un-staking partially
		}
		switch choice {
		case 0: // partially un-staking
			d.TotalPower -= rand.Int63n(d.TotalPower - 1)
			retAllDelegatees = append(retAllDelegatees, d)
		case 1: // un-staking all
			d.TotalPower = 0
		case 2: // additional staking
			d.TotalPower += rand.Int63n(math.MaxInt64 - d.TotalPower)
			retAllDelegatees = append(retAllDelegatees, d)
		case 3: // new staking
			pubBytes := secp256k1.GenPrivKey().PubKey().Bytes()
			addr, _ := crypto.PubBytes2Addr(pubBytes)
			newone := NewDelegatee(addr, pubBytes)
			newone.TotalPower = rand.Int63n(math.MaxInt64 - 1000000)
			retAllDelegatees = append(retAllDelegatees, newone)
		}
	}
	allDelegatees = retAllDelegatees

	for _, a := range allDelegatees {
		require.NotEqual(t, int64(0), a.TotalPower)
	}

	// check order by power
	sort.Sort(PowerOrderDelegatees(allDelegatees))
	prePower = int64(0)
	for i, d := range allDelegatees {
		if i == 0 {
			prePower = d.TotalPower
		} else {
			require.True(t, prePower >= d.TotalPower)
		}
	}

	// select validator set
	n = libs.MinInt(maxValCnt, len(allDelegatees))
	newVals2 := selectValidators(PowerOrderDelegatees(allDelegatees), n)
	require.NotNil(t, newVals2, "validator count", n, "allDelegatees", allDelegatees)

	sort.Sort(AddressOrderDelegatees(lastValidators))
	sort.Sort(AddressOrderDelegatees(newVals2))
	valUps = validatorUpdates(lastValidators, newVals2)

	for _, v := range valUps {
		if v.Power == 0 {
			// remove
			found := false
			for i := 0; i < len(lastValidators); i++ {
				if bytes.Compare(v.PubKey.GetSecp256K1(), lastValidators[i].PubKey) == 0 {
					lastValidators = append(lastValidators[:i], lastValidators[i+1:]...)
					found = true
					break
				}
			}
			require.True(t, found)
		} else {
			// newer or updated
			found := false
			for i := 0; i < len(lastValidators); i++ {
				if bytes.Compare(v.PubKey.GetSecp256K1(), lastValidators[i].PubKey) == 0 {
					// updated
					lastValidators[i].TotalPower = v.Power
					found = true
					break
				}
			}
			if !found {
				// newer
				pubBytes := v.PubKey.GetSecp256K1()
				addr, _ := crypto.PubBytes2Addr(pubBytes)
				d := NewDelegatee(addr, pubBytes)
				d.TotalPower = v.Power
				lastValidators = append(lastValidators, d)
			}
		}
	}
	sort.Sort(AddressOrderDelegatees(lastValidators))

	require.Equal(t, newVals2, lastValidators)
	for i, _ := range newVals2 {
		require.Equal(t, newVals2[i].Addr, lastValidators[i].Addr)
		require.Equal(t, newVals2[i].TotalPower, lastValidators[i].TotalPower)
	}

	sort.Sort(PowerOrderDelegatees(allDelegatees))
	sort.Sort(PowerOrderDelegatees(lastValidators))
	for i, _ := range lastValidators {
		require.Equal(t, allDelegatees[i].Addr, lastValidators[i].Addr)
		require.Equal(t, allDelegatees[i].TotalPower, lastValidators[i].TotalPower)
	}
}
