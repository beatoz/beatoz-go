# Governance Parameters

```go
type GovParams struct {
	version               int64
	maxValidatorCnt       int64
	minValidatorStake     *uint256.Int
	minDelegatorStake     *uint256.Int
	rewardPerPower        *uint256.Int
	lazyRewardBlocks      int64
	lazyApplyingBlocks    int64
	gasPrice              *uint256.Int
	minTrxGas             uint64
	maxTrxGas             uint64
	maxBlockGas           uint64
	minVotingPeriodBlocks int64
	maxVotingPeriodBlocks int64

	minSelfStakeRatio       int64
	maxUpdatableStakeRatio  int64
	maxIndividualStakeRatio int64
	slashRatio              int64
	signedBlocksWindow      int64
	minSignedBlocks         int64

	mtx sync.RWMutex
}
```
- **version** Governance parameters 버전.
- **maxValidatorCnt** 최대 Validator Node 개수
- **minValidatorStake** Validator 가 되기 위해 지분 전화시 최소 지분량
- **minDelegatorStake** 지분 위임시 위임하는 최소 지분량.
- **rewardPerPower** Voting Power 당 보상 수량.
- **~~lazyRewardBlocks~~**
- **lazyUnstakingBlocks** 지분을 다시 자산으로 전화할 때 지연 블록 수.
- **lazyApplyingBlocks** 새로운 거버넌스 파라메터 적용시 지연 블록 수.
- **gasPrice**
- **minTrxGas** 트랜잭션 최소 Gas 수량.
- **maxTrxGas** 트랜잭션 최대 Gas 수량.
- **maxBlockGas** 하나의 블록에 포함되는 트랜잭션 Gas 최대 수량.
- **minVotingPeriodBlocks** 투표 제안시 최소 투표 기간(블록수).
- **maxVotingPeriodBlocks** 투표 제안시 최대 투표 기간(블록수).

- **minSelfStakeRatio** Validator 지분 구성중 최소 자기 지분 비율.
- **maxUpdatableStakeRatio** Validator 변경시 변경될 수 있는 최대 지분 수량.
- **maxIndividualStakeRatio**
- **slashRatio** 비잔틴 Validator 처벌시 소각 되는 지분 비율.
- **signedBlocksWindow** 비잔틴 Validator 처벌시 소각 되는 지분 비율.
- **minSignedBlocks** 비잔틴 Validator 처벌시 소각 되는 지분 비율.
