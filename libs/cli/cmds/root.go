package cmds

import (
	"encoding/hex"
	"github.com/beatoz/beatoz-go/libs/cli/types"
	"github.com/beatoz/beatoz-go/types/bytes"
	"github.com/mitchellh/mapstructure"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"reflect"
	"strings"
)

var (
	rootFlags = &types.RootFlags{}
	RootCmd   = &cobra.Command{
		Use:   "rg",
		Short: "Command Line Tool for BEATOZ",
		PersistentPreRunE: func(cmd *cobra.Command, args []string) (err error) {
			if err = viper.BindPFlags(cmd.Flags()); err != nil {
				return err
			}
			if err = viper.Unmarshal(rootFlags, viper.DecodeHook(BytesDecodeHookFunc())); err != nil {
				return err
			}
			return nil
		},
	}
)

// PrepareBaseCmd is meant for tendermint and other servers
func PrepareBaseCmd(cmd *cobra.Command) error {
	cobra.OnInitialize(func() {})
	cmd.PersistentFlags().String("rpc", "http://localhost:26657", "RPC Node URL")
	//cmd.PersistentFlags().String("log_level", "info", "log level")
	cmd.PersistentFlags().StringP("from", "f", "", "wallet key file of sender")
	cmd.PersistentFlags().BytesHexP("to", "t", bytes.ZeroBytes(20), "an address of receiver")
	cmd.PersistentFlags().Uint64P("gas", "g", 0, "gas")
	cmd.PersistentFlags().StringP("gas_price", "p", "0", "gas price")
	cmd.PersistentFlags().StringP("amt", "a", "0", "amount to transfer")
	return nil
}

func BytesDecodeHookFunc() mapstructure.DecodeHookFuncType {
	return func(
		f reflect.Type,
		t reflect.Type,
		data interface{},
	) (interface{}, error) {
		// Check that the data is string
		if f.Kind() != reflect.String {
			return data, nil
		}

		// Check that the target type is our custom type
		if t != reflect.TypeOf([]byte{}) {
			return data, nil
		}

		// Return the parsed value
		hexStr := data.(string)
		hexStr = strings.TrimPrefix(hexStr, "0x")
		return hex.DecodeString(hexStr)
	}
}
