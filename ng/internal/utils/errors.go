package utils

import (
	"fmt"
	"strings"

	ctrl "sigs.k8s.io/controller-runtime"
)

// withReconcileResult is an error that also carries a ctrl.Result value.
type withReconcileResult struct {
	cause  error
	result ctrl.Result
}

func (w *withReconcileResult) Error() string {
	return w.cause.Error()
}

func (w *withReconcileResult) ReconcileResult() ctrl.Result {
	return w.result
}

func (w *withReconcileResult) Unwrap() error {
	return w.cause
}

func (w *withReconcileResult) Cause() error {
	return w.cause
}

// ReconcileResultOf returns with `err` and the ctrl.Result value attached to it, if it has one. Otherwise it returns with an empty Result.
func ReconcileResultOf(err error) (ctrl.Result, error) {
	type resulter interface {
		ReconcileResult() ctrl.Result
	}
	type causer interface {
		Cause() error
	}

	err2 := err
	for err2 != nil {
		if resulter, ok := err2.(resulter); ok {
			return resulter.ReconcileResult(), nil
		}
		cause, ok := err2.(causer)
		if !ok {
			break
		}
		err2 = cause.Cause()
	}
	return ctrl.Result{}, err
}

func WithReconcileResult(err error, result ctrl.Result) error {
	if err != nil {
		return &withReconcileResult{
			cause:  err,
			result: result,
		}
	}
	return nil
}

// CombinedError is an error that combines multiple errors into one.
type CombinedError struct {
	Errors []error
	Joiner string
}

var _ error = &CombinedError{}

func NewEmptyCombinedError(joiner string) *CombinedError {
	return &CombinedError{Joiner: joiner}
}

func (e *CombinedError) Error() string {
	if e == nil {
		return ""
	}
	var errorMessages []string
	for _, err := range e.Errors {
		errorMessages = append(errorMessages, err.Error())
	}
	return strings.Join(errorMessages, e.Joiner)
}

func (e *CombinedError) Add(err error) {
	e.Errors = append(e.Errors, err)
}

func (e *CombinedError) Addf(fmtString string, args ...any) {
	e.Errors = append(e.Errors, fmt.Errorf(fmtString, args...))
}

func (e *CombinedError) IsEmpty() bool {
	return e == nil || len(e.Errors) == 0
}

func (e *CombinedError) ErrorOrNil(prefix string) error {
	if e.IsEmpty() {
		return nil
	}
	if prefix == "" {
		return e
	}
	return fmt.Errorf("%s%s%w", prefix, e.Joiner, e)
}

func CombineErrors(errors []error, joiner string) error {
	if len(errors) == 0 {
		return nil
	}
	return &CombinedError{Errors: errors, Joiner: joiner}
}
