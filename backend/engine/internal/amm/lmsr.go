package amm

import (
	"math"
)

// CalculateCost computes the total cost function for an LMSR AMM
// C = b * ln(e^(q_yes/b) + e^(q_no/b))
func CalculateCost(qYes, qNo, bParameter float64) float64 {
	expYes := math.Exp(qYes / bParameter)
	expNo := math.Exp(qNo / bParameter)
	return bParameter * math.Log(expYes+expNo)
}

// CalculatePrice computes the instantaneous price of a given outcome
// P_yes = e^(q_yes/b) / (e^(q_yes/b) + e^(q_no/b))
func CalculatePrice(qYes, qNo, bParameter float64, isYes bool) float64 {
	expYes := math.Exp(qYes / bParameter)
	expNo := math.Exp(qNo / bParameter)
	
	if isYes {
		return expYes / (expYes + expNo)
	}
	return expNo / (expYes + expNo)
}

// CalculateCostForShares calculates the exact cost for purchasing `deltaShares` of an outcome
// Cost = C(q_yes + delta, q_no) - C(q_yes, q_no)
func CalculateCostForShares(qYes, qNo, bParameter, deltaShares float64, isYes bool) float64 {
	initialCost := CalculateCost(qYes, qNo, bParameter)
	
	var finalCost float64
	if isYes {
		finalCost = CalculateCost(qYes+deltaShares, qNo, bParameter)
	} else {
		finalCost = CalculateCost(qYes, qNo+deltaShares, bParameter)
	}
	
	return finalCost - initialCost
}
