package toolslib

import (
	"github.com/btcsuite/btcd/btcec"
	"github.com/deso-protocol/core/lib"
	"github.com/tyler-smith/go-bip39"
)

// GenerateMnemonicPublicPrivate,,,
func GenerateMnemonicPublicPrivate(params *lib.DeSoParams) (mnemonic string, pubKey *btcec.PublicKey, privKey *btcec.PrivateKey) {
	entropy, _ := bip39.NewEntropy(128)
	mnemonic, _ = bip39.NewMnemonic(entropy)
	seedBytes, _ := bip39.NewSeedWithErrorChecking(mnemonic, "")
	pubKey, privKey, _, _ = lib.ComputeKeysFromSeed(seedBytes, 0, params)
	return
}
