package handler

import "errors"

var errReadinessUnavailable = errors.New("readiness unavailable")

func joinReadiness(left, right error) error {
	if left != nil {
		return left
	}
	return right
}
