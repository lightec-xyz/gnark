// Copyright 2020 ConsenSys Software Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Code generated by gnark DO NOT EDIT

package plonk

import (
	"github.com/consensys/gnark/crypto/polynomial"
	"github.com/consensys/gnark/crypto/polynomial/bw761"
	"github.com/consensys/gnark/frontend"
	"github.com/consensys/gnark/internal/backend/bw761/cs"
	"github.com/consensys/gnark/internal/backend/bw761/fft"
	bw761witness "github.com/consensys/gnark/internal/backend/bw761/witness"
	"github.com/consensys/gurvy/bw761/fr"
)

// PublicRaw represents the raw public data corresponding to a circuit,
// which consists of the evaluations of the LDE of qr,ql,qm,qo,k. The compact
// version of public data consists of commitments of qr,ql,qm,qo,k.
type PublicRaw struct {

	// Commitment scheme that is used for an instantiation of PLONK
	CommitmentScheme polynomial.CommitmentScheme

	// qr,ql,qm,qo,k (in canonical basis)
	Ql, Qr, Qm, Qo, Qk bw761.Poly

	// Domains used for the FFTs
	DomainNum, DomainH *fft.Domain

	// shifters for extending the permutation set: from s=<1,z,..,z**n-1>,
	// extended domain = s || shifter[0].s || shifter[1].s
	Shifter [2]fr.Element

	// s1, s2, s3 (L=Lagrange basis, C=canonical basis)
	LS1, LS2, LS3 bw761.Poly
	CS1, CS2, CS3 bw761.Poly

	// position -> permuted position (position in [0,3*sizeSystem-1])
	Permutation []int
}

// SetupRaw from a sparseR1CS
// * sets LDE+canonical basis representations of the permutations
// * sets the canonical basis of ql, qr, qm, qo, qk extended (i.e. containing also placeholders constraints -PUB_INPUT_i + qk_i=0)
// * sets the fft domains that will be needed for handling polynomials
// The publicWitness params is here to build the placeholder constraints (used in the verifier to complete the proof)
// TODO in many places this function should handle raising errors
func SetupRaw(spr *cs.SparseR1CS, polynomialCommitment polynomial.CommitmentScheme, publicWitness frontend.Circuit) *PublicRaw {

	wPublic := bw761witness.Witness{}
	// TODO handle error here
	wPublic.FromPublicAssignment(publicWitness)

	nbConstraints := len(spr.Constraints)
	nbAssertions := len(spr.Assertions)

	var res PublicRaw

	// fft domains
	sizeSystem := uint64(nbConstraints + nbAssertions + spr.NbPublicVariables) // spr.NbPublicVariables is for the placeholder constraints
	res.DomainNum = fft.NewDomain(sizeSystem, 3)
	res.DomainH = fft.NewDomain(4*sizeSystem, 1)

	// shifters
	res.Shifter[0].Set(&res.DomainNum.FinerGenerator)
	res.Shifter[1].Square(&res.DomainNum.FinerGenerator)

	// commitment scheme
	res.CommitmentScheme = polynomialCommitment

	// public polynomials corresponding to constraints: [ placholders | constraints | assertions ]
	res.Ql = make([]fr.Element, res.DomainNum.Cardinality)
	res.Qr = make([]fr.Element, res.DomainNum.Cardinality)
	res.Qm = make([]fr.Element, res.DomainNum.Cardinality)
	res.Qo = make([]fr.Element, res.DomainNum.Cardinality)
	res.Qk = make([]fr.Element, res.DomainNum.Cardinality)

	for i := 0; i < spr.NbPublicVariables; i++ { // placeholders (-PUB_INPUT_i + qk_i = 0) TODO should return error is size is inconsistant
		res.Ql[i].SetOne().Neg(&res.Ql[i])
		res.Qr[i].SetZero()
		res.Qm[i].SetZero()
		res.Qo[i].SetZero()
		res.Qk[i].Set(&wPublic[i])
	}
	offset := spr.NbPublicVariables
	for i := 0; i < nbConstraints; i++ { // constraints

		res.Ql[offset+i].Set(&spr.Coefficients[spr.Constraints[i].L.CoeffID()])
		res.Qr[offset+i].Set(&spr.Coefficients[spr.Constraints[i].R.CoeffID()])
		res.Qm[offset+i].Set(&spr.Coefficients[spr.Constraints[i].M[0].CoeffID()]).
			Mul(&res.Qm[offset+i], &spr.Coefficients[spr.Constraints[i].M[1].CoeffID()])
		res.Qo[offset+i].Set(&spr.Coefficients[spr.Constraints[i].O.CoeffID()])
		res.Qk[offset+i].Set(&spr.Coefficients[spr.Constraints[i].K])
	}
	offset += nbConstraints
	for i := 0; i < nbAssertions; i++ { // assertions

		res.Ql[offset+i].Set(&spr.Coefficients[spr.Assertions[i].L.CoeffID()])
		res.Qr[offset+i].Set(&spr.Coefficients[spr.Assertions[i].R.CoeffID()])
		res.Qm[offset+i].Set(&spr.Coefficients[spr.Assertions[i].M[0].CoeffID()]).
			Mul(&res.Qm[offset+i], &spr.Coefficients[spr.Assertions[i].M[1].CoeffID()])
		res.Qo[offset+i].Set(&spr.Coefficients[spr.Assertions[i].O.CoeffID()])
		res.Qk[offset+i].Set(&spr.Coefficients[spr.Assertions[i].K])
	}

	res.DomainNum.FFTInverse(res.Ql, fft.DIF, 0)
	res.DomainNum.FFTInverse(res.Qr, fft.DIF, 0)
	res.DomainNum.FFTInverse(res.Qm, fft.DIF, 0)
	res.DomainNum.FFTInverse(res.Qo, fft.DIF, 0)
	res.DomainNum.FFTInverse(res.Qk, fft.DIF, 0)
	fft.BitReverse(res.Ql)
	fft.BitReverse(res.Qr)
	fft.BitReverse(res.Qm)
	fft.BitReverse(res.Qo)
	fft.BitReverse(res.Qk)

	// build permutation. Note: at this stage, the permutation takes in account the placeholders
	buildPermutation(spr, &res)

	// set s1, s2, s3
	ComputeS(&res)

	return &res
}

// buildPermutation builds the Permutation associated with a circuit.
//
// The permutation s is composed of cycles of maximum length such that
//
// 			s. (l||r||o) = (l||r||o)
//
//, where l||r||o is the concatenation of the indices of l, r, o in
// ql.l+qr.r+qm.l.r+qo.O+k = 0.
//
// The permutation is encoded as a slice s of size 3*size(l), where the
// i-th entry of l||r||o is sent to the s[i]-th entry, so it acts on a tab
// like this: for i in tab: tab[i] = tab[permutation[i]]
func buildPermutation(spr *cs.SparseR1CS, publicData *PublicRaw) {

	sizeSolution := int(publicData.DomainNum.Cardinality)

	// position -> variable_ID
	lro := make([]int, 3*sizeSolution)

	publicData.Permutation = make([]int, 3*sizeSolution)
	for i := 0; i < spr.NbPublicVariables; i++ { // IDs of LRO associated to placeholders (only L needs to be taken care of)

		lro[i] = i
		lro[sizeSolution+i] = 0
		lro[2*sizeSolution+i] = 0

		publicData.Permutation[i] = -1
		publicData.Permutation[sizeSolution+i] = -1
		publicData.Permutation[2*sizeSolution+i] = -1
	}
	offset := spr.NbPublicVariables
	for i := 0; i < len(spr.Constraints); i++ { // IDs of LRO associated to constraints

		lro[offset+i] = spr.Constraints[i].L.VariableID()
		lro[sizeSolution+offset+i] = spr.Constraints[i].R.VariableID()
		lro[2*sizeSolution+offset+i] = spr.Constraints[i].O.VariableID()

		publicData.Permutation[i+offset] = -1
		publicData.Permutation[sizeSolution+i+offset] = -1
		publicData.Permutation[2*sizeSolution+i+offset] = -1
	}
	offset += len(spr.Constraints)
	for i := 0; i < len(spr.Assertions); i++ { // IDs of LRO associated to assertions

		lro[offset+i] = spr.Assertions[i].L.VariableID()
		lro[offset+sizeSolution+i] = spr.Assertions[i].R.VariableID()
		lro[offset+2*sizeSolution+i] = spr.Assertions[i].O.VariableID()

		publicData.Permutation[offset+i] = -1
		publicData.Permutation[offset+sizeSolution+i] = -1
		publicData.Permutation[offset+2*sizeSolution+i] = -1
	}
	offset += len(spr.Assertions)
	for i := 0; i < sizeSolution-offset; i++ {

		publicData.Permutation[offset+i] = -1
		publicData.Permutation[offset+sizeSolution+i] = -1
		publicData.Permutation[offset+2*sizeSolution+i] = -1
	}

	nbVariables := spr.NbInternalVariables + spr.NbPublicVariables + spr.NbSecretVariables

	// map ID -> last position the ID was seen
	cycle := make([]int, nbVariables)
	for i := 0; i < len(cycle); i++ {
		cycle[i] = -1
	}

	for i := 0; i < 3*sizeSolution; i++ {
		if cycle[lro[i]] != -1 {
			publicData.Permutation[i] = cycle[lro[i]]
		}
		cycle[lro[i]] = i
	}

	// complete the Permutation by filling the first IDs encountered
	counter := nbVariables
	for iter := 0; counter > 0; iter++ {
		if publicData.Permutation[iter] == -1 {
			publicData.Permutation[iter] = cycle[lro[iter]]
			counter--
		}
	}

}

// ComputeS computes the LDE (Lagrange basis) of the permutations
// s1, s2, s3.
//
// ex: z gen of Z/mZ, u gen of Z/8mZ, then
//
// 1	z 	..	z**n-1	|	u	uz	..	u*z**n-1	|	u**2	u**2*z	..	u**2*z**n-1  |
//  																					 |
//        																				 | Permutation
// s11  s12 ..   s1n	   s21 s22 	 ..		s2n		     s31 	s32 	..		s3n		 v
// \---------------/       \--------------------/        \------------------------/
// 		s1 (LDE)                s2 (LDE)                          s3 (LDE)
func ComputeS(publicData *PublicRaw) {

	nbElmt := int(publicData.DomainNum.Cardinality)

	// sID = [1,z,..,z**n-1,u,uz,..,uz**n-1,u**2,u**2.z,..,u**2.z**n-1]
	sID := make([]fr.Element, 3*nbElmt)
	sID[0].SetOne()
	sID[nbElmt].Set(&publicData.DomainNum.FinerGenerator)
	sID[2*nbElmt].Square(&publicData.DomainNum.FinerGenerator)

	for i := 1; i < nbElmt; i++ {
		sID[i].Mul(&sID[i-1], &publicData.DomainNum.Generator)                   // z**i -> z**i+1
		sID[i+nbElmt].Mul(&sID[nbElmt+i-1], &publicData.DomainNum.Generator)     // u*z**i -> u*z**i+1
		sID[i+2*nbElmt].Mul(&sID[2*nbElmt+i-1], &publicData.DomainNum.Generator) // u**2*z**i -> u**2*z**i+1
	}

	// Lagrange form of S1, S2, S3
	publicData.LS1 = make(bw761.Poly, nbElmt)
	publicData.LS2 = make(bw761.Poly, nbElmt)
	publicData.LS3 = make(bw761.Poly, nbElmt)
	for i := 0; i < nbElmt; i++ {
		publicData.LS1[i].Set(&sID[publicData.Permutation[i]])
		publicData.LS2[i].Set(&sID[publicData.Permutation[nbElmt+i]])
		publicData.LS3[i].Set(&sID[publicData.Permutation[2*nbElmt+i]])
	}

	// Canonical form of S1, S2, S3
	publicData.CS1 = make(bw761.Poly, nbElmt)
	publicData.CS2 = make(bw761.Poly, nbElmt)
	publicData.CS3 = make(bw761.Poly, nbElmt)
	copy(publicData.CS1, publicData.LS1)
	copy(publicData.CS2, publicData.LS2)
	copy(publicData.CS3, publicData.LS3)
	publicData.DomainNum.FFTInverse(publicData.CS1, fft.DIF, 0)
	publicData.DomainNum.FFTInverse(publicData.CS2, fft.DIF, 0)
	publicData.DomainNum.FFTInverse(publicData.CS3, fft.DIF, 0)
	fft.BitReverse(publicData.CS1)
	fft.BitReverse(publicData.CS2)
	fft.BitReverse(publicData.CS3)

}
