// accepting request and finding function to handle it (look through research to find something that takes a lot of time)
package matrix

import (
	"encoding/json"
	"fmt"
	"log"
	"math"
	"net/http"
)

type MatrixRequest struct {
	Matrix [][]float64 `json:"matrix"`
}

type MatrixResponse struct {
	LowerTriangular [][]float64 `json:"lower_triangular"`
}

// Cholesky Factorization Function
func choleskyFactorization(matrix [][]float64) ([][]float64, error) {
	n := len(matrix)
	L := make([][]float64, n)
	for i := range L {
		L[i] = make([]float64, n)
	}

	for i := 0; i < n; i++ {
		for j := 0; j <= i; j++ {
			sum := 0.0
			for k := 0; k < j; k++ {
				sum += L[i][k] * L[j][k]
			}

			if i == j {
				val := matrix[i][i] - sum
				if val <= 0 {
					return nil, fmt.Errorf("matrix is not positive definite")
				}
				L[i][j] = math.Sqrt(val)
			} else {
				L[i][j] = (matrix[i][j] - sum) / L[j][j]
			}
		}
	}

	return L, nil
}

// Handler for Cholesky Factorization
func handleCholesky(w http.ResponseWriter, r *http.Request) {
	var req MatrixRequest
	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		http.Error(w, "Invalid input", http.StatusBadRequest)
		return
	}

	L, err := choleskyFactorization(req.Matrix)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	res := MatrixResponse{LowerTriangular: L}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(res)
}

func main() {
	http.HandleFunc("/cholesky", handleCholesky)
	// can add middleware over here
	fmt.Println("Server is running on port 8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}
