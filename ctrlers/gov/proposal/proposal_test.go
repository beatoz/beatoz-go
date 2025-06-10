package proposal

import (
	"github.com/beatoz/beatoz-go/libs/jsonx"
	"github.com/beatoz/beatoz-go/types"
	"github.com/beatoz/beatoz-go/types/bytes"
	"github.com/stretchr/testify/require"
	"testing"
)

func Test_Proposal_Codec(t *testing.T) {
	prop := NewGovProposal(
		PROPOSAL_ONCHAIN,
		bytes.RandBytes(32),
		123,
		456,
		789,
		321,
	)

	prop.AddVoter(types.RandAddress(), 13423)
	prop.AddVoter(types.RandAddress(), 13423)
	prop.AddVoter(types.RandAddress(), 13423)
	prop.AddOption([]byte(`{"name":"test","value":"test"}`))

	jz, err := jsonx.Marshal(prop)
	require.NoError(t, err)

	//fmt.Println(string(jz))

	prop2 := &GovProposal{}
	err = jsonx.Unmarshal(jz, prop2)
	require.NoError(t, err)

	require.Equal(t, prop, prop2)
}
