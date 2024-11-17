package matrix

import "testing"

func TestCholeskyFactorization(t *testing.T) {
	matrix := [][]float64{
		{4, 12, -16},
		{12, 37, -43},
		{-16, -43, 98},
	}
	_, err := CholeskyFactorization(matrix)
	if err != nil {
		t.Fatalf("Failed Cholesky factorization: %v", err)
	}
	t.Log("Cholesky factorization passed")
}
