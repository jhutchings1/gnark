package merkle

import (
	"bytes"
	"fmt"
	"math/big"
	"os"
	"testing"

	backend_common "github.com/consensys/gnark/backend"
	backend_bn256 "github.com/consensys/gnark/backend/bn256"
	"github.com/consensys/gnark/backend/bn256/groth16"
	"github.com/consensys/gnark/crypto/hash/mimc/bn256"
	"github.com/consensys/gnark/frontend"
	"github.com/consensys/gnark/gadgets/hash/mimc"
	"github.com/consensys/gurvy"
	"github.com/consensys/gurvy/bn256/fr"
	"gitlab.com/NebulousLabs/merkletree"
)

func TestLeaf(t *testing.T) {

	// computation of expected result
	leafPrefix := []byte{0x00}
	var leaf fr.Element
	//leaf.SetRandom()
	leaf.SetString("452312848583266388373314160190187140051835878600158453279131187530910662657")
	h := bn256.NewMiMC("seed")
	h.Write(leafPrefix)
	h.Write(leaf.Bytes())
	binExpectedRes := h.Sum([]byte{})

	var tmp fr.Element
	tmp.SetBytes(binExpectedRes)
	expectedRes := make(map[string]fr.Element)
	expectedRes["res"] = tmp

	// computation of leafNode using gadget
	circuit := frontend.New()
	hg, err := mimc.NewMiMCGadget("seed", gurvy.BN256)
	if err != nil {
		t.Fatal(err)
	}
	res := leafSum(&circuit, hg, circuit.SECRET_INPUT("leaf"))
	res.Tag("res")
	assignment := backend_common.NewAssignment()
	assignment.Assign(backend_common.Secret, "leaf", leaf)

	r1cs := backend_bn256.New(&circuit)

	assert := groth16.NewAssert(t)
	assert.CorrectExecution(&r1cs, assignment, expectedRes)
}

func TestNode(t *testing.T) {

	var shifter big.Int
	shifter.SetString("452312848583266388373324160190187140051835877600158453279131187530910662656", 10)

	// computation of expected result
	nodePrefix := []byte{0x01}
	var node1, node2 fr.Element
	node1.SetRandom()
	node2.SetRandom()
	h := bn256.NewMiMC("seed")
	h.Write(nodePrefix)
	h.Write(node1.Bytes())
	h.Write(node2.Bytes())
	binExpectedRes := h.Sum([]byte{})
	var tmp fr.Element
	tmp.SetBytes(binExpectedRes)
	expectedRes := make(map[string]fr.Element)
	expectedRes["res"] = tmp

	// computation of leafNode using gadget
	circuit := frontend.New()
	hg, err := mimc.NewMiMCGadget("seed", gurvy.BN256)
	if err != nil {
		t.Fatal(err)
	}
	res := nodeSum(&circuit, hg, circuit.SECRET_INPUT("node1"), circuit.SECRET_INPUT("node2"))
	res.Tag("res")
	assignment := backend_common.NewAssignment()
	assignment.Assign(backend_common.Secret, "node1", node1)
	assignment.Assign(backend_common.Secret, "node2", node2)

	r1cs := backend_bn256.New(&circuit)

	assert := groth16.NewAssert(t)
	assert.CorrectExecution(&r1cs, assignment, expectedRes)
}

// TODO need tests
func TestVerify(t *testing.T) {

	t.Skip("wip")

	// generate random data
	// makes sure that each chunk of 64 bits fits in a fr modulus, otherwise there are bugs due to the padding (domain separation)
	var buf bytes.Buffer
	for i := 0; i < 32; i++ {
		var leaf fr.Element
		leaf.SetRandom()
		fmt.Fprintf(&buf, string(leaf.Bytes()))
	}

	// build & verify proof for an elmt in the file
	proofIndex := uint64(7)
	segmentSize := 32
	merkleRoot, proof, numLeaves, err := merkletree.BuildReaderProof(&buf, bn256.NewMiMC("seed"), segmentSize, proofIndex)
	if err != nil {
		fmt.Println(err)
		os.Exit(-1)
	}

	verified := merkletree.VerifyProof(bn256.NewMiMC("seed"), merkleRoot, proof, proofIndex, numLeaves)
	if !verified {
		t.Fatal("The merkle proof in plain go should pass")
	}

	// create circuit
	circuit := frontend.New()

	// public root hash
	rh := circuit.PUBLIC_INPUT("rootHash")

	// private
	priv := make([]*frontend.Constraint, 0)
	for i := 0; i < len(proof); i++ {
		tmp := circuit.SECRET_INPUT("path" + string(i))
		priv = append(priv, tmp)
	}

	hFunc, err := mimc.NewMiMCGadget("seed", gurvy.BN256)
	if err != nil {
		t.Fatal(err)
	}
	VerifyProof(&circuit, hFunc, rh, priv, proofIndex, numLeaves)

	// compilation of the circuit
	r1cs := backend_bn256.New(&circuit)

	assignment := backend_common.NewAssignment()
	assignment.Assign(backend_common.Public, "rootHash", merkleRoot)
	for i := 0; i < len(proof); i++ {
		assignment.Assign(backend_common.Secret, "path"+string(i), proof[i])
	}

	fmt.Println("--")
	res, _ := r1cs.Inspect(assignment, false)
	for k, v := range res {
		fmt.Println(k + ": " + v.String())
	}

	// assert := groth16.NewAssert(t)
	// assert.Solved(&r1cs, assignment, nil)
}
