package vpower

//type VPowerHandlerMock struct {
//	valCnt     int
//	delegatees []*vpower.Delegatee
//	vpows      map[string]*vpower.VPower
//	totalPower int64 // key is from_address + to_address
//}
//
//func NewVPowerHandlerMock(valCnt int) *VPowerHandlerMock {
//	//validators := make([]*vpower.Delegatee, valCnt)
//	//for i := 0; i < valCnt; i++ {
//	//	_, pub := crypto.NewKeypairBytes()
//	//	validators[i] = vpower.NewDelegatee(pub)
//	//
//	//	vpowCnt := rand.Intn(10000)
//	//	for j := 0; j < vpowCnt; j++ {
//	//		vpow := vpower.NewVPower(types.RandAddress(), pub)
//	//		vpow.PowerChunks
//	//	}
//	//}
//	return nil
//}
//
//func (mock *VPowerHandlerMock) ComputeWeight(height, ripeningBlocks, tau int64, totalSupply *uint256.Int) (decimal.Decimal, xerrors.XError) {
//	return decimal.Zero, nil
//}
//
//func (mock *VPowerHandlerMock) Validators() ([]*abcitypes.Validator, int64) {
//	totalPower := int64(0)
//	vals := make([]*abcitypes.Validator, mock.valCnt)
//	for i := 0; i < mock.valCnt; i++ {
//		vals[i] = &abcitypes.Validator{
//			Address: mock.delegatees[i].Address(),
//			Power:   mock.delegatees[i].SumPower,
//		}
//		totalPower += mock.delegatees[i].SumPower
//	}
//	return vals, totalPower
//}
//
//func (mock *VPowerHandlerMock) IsValidator(addr types.Address) bool {
//	for i := 0; i < mock.valCnt; i++ {
//		if bytes.Compare(addr, mock.delegatees[i].Address()) == 0 {
//			return true
//		}
//	}
//	return false
//}
//
//func (mock *VPowerHandlerMock) GetTotalAmount() *uint256.Int {
//	return ctrlertypes.PowerToAmount(mock.GetTotalPower())
//}
//
//func (mock *VPowerHandlerMock) GetTotalPower() int64 {
//	sum := int64(0)
//	for _, v := range mock.delegatees {
//		sum += v.SumPower
//	}
//	return sum
//}
//
//func (mock *VPowerHandlerMock) TotalPowerOf(addr types.Address) int64 {
//	for _, v := range mock.delegatees {
//		if bytes.Compare(addr, v.Address()) == 0 {
//			return v.SumPower
//		}
//	}
//	return int64(0)
//}
//
//func (mock *VPowerHandlerMock) SelfPowerOf(addr types.Address) int64 {
//	return 0
//}
//
//func (mock *VPowerHandlerMock) DelegatedPowerOf(addr types.Address) int64 {
//	return 0
//}
//
//func (mock *VPowerHandlerMock) PickAddress(i int) types.Address {
//	return mock.delegatees[i].Address()
//}
//
//var _ ctrlertypes.IVPowerHandler = (*VPowerHandlerMock)(nil)
