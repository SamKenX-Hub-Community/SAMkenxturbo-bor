package ethash

import (
	"time"

	mapset "github.com/deckarep/golang-set"

	"github.com/ledgerwatch/erigon/common"
	"github.com/ledgerwatch/erigon/consensus"
	"github.com/ledgerwatch/erigon/core/types"
	"github.com/ledgerwatch/log/v3"
)

type FakeEthash struct {
	Ethash
	fakeFail  uint64        // Block number which fails PoW check even in fake mode
	fakeDelay time.Duration // Time delay to sleep for before returning from verify
}

// NewFakeFailer creates a ethash consensus engine with a fake PoW scheme that
// accepts all blocks as valid apart from the single one specified, though they
// still have to conform to the Ethereum consensus rules.
func NewFakeFailer(fail uint64) *FakeEthash {
	return &FakeEthash{
		Ethash:   newFakeEth(ModeFake),
		fakeFail: fail,
	}
}

// NewFaker creates a ethash consensus engine with a fake PoW scheme that accepts
// all blocks' seal as valid, though they still have to conform to the Ethereum
// consensus rules.
func NewFaker() *FakeEthash {
	return &FakeEthash{
		Ethash: newFakeEth(ModeFake),
	}
}

// NewFakeDelayer creates a ethash consensus engine with a fake PoW scheme that
// accepts all blocks as valid, but delays verifications by some time, though
// they still have to conform to the Ethereum consensus rules.
func NewFakeDelayer(delay time.Duration) *FakeEthash {
	return &FakeEthash{
		Ethash:    newFakeEth(ModeFake),
		fakeDelay: delay,
	}
}

func newFakeEth(mode Mode) Ethash {
	return Ethash{
		config: Config{
			PowMode: mode,
			Log:     log.Root(),
		},
	}
}

func (f *FakeEthash) VerifyHeaders(chain consensus.ChainHeaderReader, headers []*types.Header, seals []bool) error {
	fakeSeals := make([]bool, len(seals))
	if err := f.Ethash.VerifyHeaders(chain, headers, fakeSeals); err != nil {
		return err
	}
	for i, header := range headers {
		if seals[i] {
			if err := f.VerifySeal(chain, header); err != nil {
				return err
			}
		}
	}
	return nil
}

func (f *FakeEthash) VerifyHeader(chain consensus.ChainHeaderReader, header *types.Header, seal bool, sysCall consensus.SystemCall) error {
	err := f.Ethash.VerifyHeader(chain, header, false, sysCall)
	if err != nil {
		return err
	}

	if seal {
		return f.VerifySeal(chain, header)
	}

	return nil
}

func (f *FakeEthash) VerifyUncles(chain consensus.ChainReader, header *types.Header, uncles []*types.Header) error {
	if len(uncles) > maxUncles {
		return errTooManyUncles
	}
	if len(uncles) == 0 {
		return nil
	}

	uncleBlocks, ancestors := getUncles(chain, header)

	for _, uncle := range uncles {
		if err := f.VerifyUncle(chain, header, uncle, uncleBlocks, ancestors, true); err != nil {
			return err
		}
	}
	return nil
}

func (f *FakeEthash) VerifyUncle(chain consensus.ChainHeaderReader, block *types.Header, uncle *types.Header, uncles mapset.Set, ancestors map[common.Hash]*types.Header, seal bool) error {
	err := f.Ethash.VerifyUncle(chain, block, uncle, uncles, ancestors, false)
	if err != nil {
		return err
	}

	if seal {
		return f.VerifySeal(chain, uncle)
	}
	return nil
}

// If we're running a fake PoW, accept any seal as valid
func (f *FakeEthash) VerifySeal(_ consensus.ChainHeaderReader, header *types.Header) error {
	if f.fakeDelay > 0 {
		time.Sleep(f.fakeDelay)
	}

	if f.fakeFail == header.Number.Uint64() {
		return errInvalidPoW
	}

	return nil
}

// If we're running a fake PoW, simply return a 0 nonce immediately
func (f *FakeEthash) Seal(_ consensus.ChainHeaderReader, block *types.Block, results chan<- *types.Block, stop <-chan struct{}) error {
	header := block.Header()
	header.Nonce, header.MixDigest = types.BlockNonce{}, common.Hash{}

	select {
	case results <- block.WithSeal(header):
	default:
		f.Ethash.config.Log.Warn("Sealing result is not read by miner", "mode", "fake", "sealhash", f.SealHash(block.Header()))
	}

	return nil
}

type FullFakeEthash FakeEthash

// NewFullFaker creates an ethash consensus engine with a full fake scheme that
// accepts all blocks as valid, without checking any consensus rules whatsoever.
func NewFullFaker() *FullFakeEthash {
	return &FullFakeEthash{
		Ethash: Ethash{
			config: Config{
				PowMode: ModeFullFake,
				Log:     log.Root(),
			},
		},
	}
}

// If we're running a full engine faking, accept any input as valid
func (f *FullFakeEthash) VerifyHeader(_ consensus.ChainHeaderReader, _ *types.Header, _ bool, sysCall consensus.SystemCall) error {
	return nil
}

// If we're running a full engine faking, accept any input as valid
func (f *FullFakeEthash) VerifyHeaders(_ consensus.ChainHeaderReader, headers []*types.Header, _ []bool) error {
	return nil
}

func (f *FullFakeEthash) VerifyUncles(_ consensus.ChainReader, _ *types.Header, _ []*types.Header) error {
	return nil
}
