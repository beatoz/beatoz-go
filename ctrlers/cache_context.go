package ctrlers

import (
	ctrlertypes "github.com/beatoz/beatoz-go/ctrlers/types"
	"github.com/beatoz/beatoz-go/types/xerrors"
)

type BlockCacheContext struct {
	origGov    ctrlertypes.IGovHandler
	origAcct   ctrlertypes.IAccountHandler
	origSupply ctrlertypes.ISupplyHandler
	origVPower ctrlertypes.IVPowerHandler

	writeCaches []func() xerrors.XError
	restored    bool
}

func NewBlockCacheContext(bctx *ctrlertypes.BlockContext, exec bool) *BlockCacheContext {
	cachedAcct, writeAcct, ok := cacheAccountHandler(bctx.AcctHandler, exec)
	if !ok {
		return nil
	}
	cachedGov, writeGov, ok := cacheGovHandler(bctx.GovHandler, exec)
	if !ok {
		return nil
	}
	cachedSupply, writeSupply, ok := cacheSupplyHandler(bctx.SupplyHandler, exec)
	if !ok {
		return nil
	}
	cachedVPower, writeVPower, ok := cacheVPowerHandler(bctx.VPowerHandler, exec)
	if !ok {
		return nil
	}

	scope := &BlockCacheContext{
		origGov:    bctx.GovHandler,
		origAcct:   bctx.AcctHandler,
		origSupply: bctx.SupplyHandler,
		origVPower: bctx.VPowerHandler,
		writeCaches: []func() xerrors.XError{
			writeAcct,
			writeGov,
			writeSupply,
			writeVPower,
		},
	}

	bctx.GovHandler = cachedGov
	bctx.AcctHandler = cachedAcct
	bctx.SupplyHandler = cachedSupply
	bctx.VPowerHandler = cachedVPower

	return scope
}

func cacheAccountHandler(handler ctrlertypes.IAccountHandler, exec bool) (ctrlertypes.IAccountHandler, func() xerrors.XError, bool) {
	if handler == nil {
		return nil, noopWriteCache, true
	}
	cacheable, ok := handler.(ctrlertypes.ICacheableAccountHandler)
	if !ok {
		return nil, nil, false
	}
	cached, writeCache := cacheable.CacheHandlerContext(exec)
	return cached, writeCache, true
}

func cacheGovHandler(handler ctrlertypes.IGovHandler, exec bool) (ctrlertypes.IGovHandler, func() xerrors.XError, bool) {
	if handler == nil {
		return nil, noopWriteCache, true
	}
	cacheable, ok := handler.(ctrlertypes.ICacheableGovHandler)
	if !ok {
		return nil, nil, false
	}
	cached, writeCache := cacheable.CacheHandlerContext(exec)
	return cached, writeCache, true
}

func cacheSupplyHandler(handler ctrlertypes.ISupplyHandler, exec bool) (ctrlertypes.ISupplyHandler, func() xerrors.XError, bool) {
	if handler == nil {
		return nil, noopWriteCache, true
	}
	cacheable, ok := handler.(ctrlertypes.ICacheableSupplyHandler)
	if !ok {
		return nil, nil, false
	}
	cached, writeCache := cacheable.CacheHandlerContext(exec)
	return cached, writeCache, true
}

func cacheVPowerHandler(handler ctrlertypes.IVPowerHandler, exec bool) (ctrlertypes.IVPowerHandler, func() xerrors.XError, bool) {
	if handler == nil {
		return nil, noopWriteCache, true
	}
	cacheable, ok := handler.(ctrlertypes.ICacheableVPowerHandler)
	if !ok {
		return nil, nil, false
	}
	cached, writeCache := cacheable.CacheHandlerContext(exec)
	return cached, writeCache, true
}

func noopWriteCache() xerrors.XError {
	return nil
}

func (scope *BlockCacheContext) Restore(bctx *ctrlertypes.BlockContext) {
	if scope.restored {
		return
	}
	bctx.GovHandler = scope.origGov
	bctx.AcctHandler = scope.origAcct
	bctx.SupplyHandler = scope.origSupply
	bctx.VPowerHandler = scope.origVPower
	scope.restored = true
}

func (scope *BlockCacheContext) Write() xerrors.XError {
	for _, writeCache := range scope.writeCaches {
		if xerr := writeCache(); xerr != nil {
			return xerr
		}
	}
	return nil
}
