package vpower

func (dgtee *Delegatee) DoSlash(ratio int64) int64 {
	dgtee.mtx.Lock()
	defer dgtee.mtx.Unlock()

	// to slash delegators too. issue #49
	return dgtee.doSlashAll(ratio)
}

// doSlashAll slashes both the validator's and the delegator's power.
func (dgtee *Delegatee) doSlashAll(ratio int64) int64 {
	sumSlashedPower := int64(0)
	// todo: Implemnet me
	//var removingVPows []*VPowerProto
	//for _, s0 := range dgtee.Stakes {
	//	slashedPower := (s0.Power * ratio) / int64(100)
	//	if slashedPower < 1 {
	//		removingStakes = append(removingStakes, s0)
	//		slashedPower = s0.Power
	//		continue
	//	}
	//
	//	s0.Power -= slashedPower
	//	sumSlashedPower += slashedPower
	//}
	//
	//if removingStakes != nil {
	//	for _, s1 := range removingStakes {
	//		_ = dgtee.delStakeByHash(s1.TxHash)
	//	}
	//}
	//
	//dgtee.SelfPower = delegatee.sumPowerOf(delegatee.Addr)
	//dgtee.TotalPower = delegatee.sumPowerOf(nil)

	return sumSlashedPower
}
