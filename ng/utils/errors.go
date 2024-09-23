package utils

import (
	"errors"
	"fmt"
	"strings"

	utilerrors "k8s.io/apimachinery/pkg/util/errors"
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

// ErrorCollector is an error that combines multiple errors into one.
type ErrorCollector struct {
	errors []error
	Joiner string
}

var _ error = &ErrorCollector{}
var _ utilerrors.Aggregate = &ErrorCollector{}

func NewErrorCollector(joiner string) *ErrorCollector {
	if joiner == "" {
		joiner = "; "
	}
	return &ErrorCollector{Joiner: joiner}
}

func (e *ErrorCollector) Errors() []error {
	return e.errors
}

func (e *ErrorCollector) Is(err error) bool {
	for _, e := range e.errors {
		if errors.Is(e, err) {
			return true
		}
	}
	return false
}

func (e *ErrorCollector) Error() string {
	if e == nil {
		return ""
	}
	var errorMessages []string
	for _, err := range e.errors {
		errorMessages = append(errorMessages, err.Error())
	}
	return strings.Join(errorMessages, e.Joiner)
}

func (e *ErrorCollector) Add(err error) {
	if err != nil {
		e.errors = append(e.errors, err)
	}
}

func (e *ErrorCollector) AddWithPrefix(err error, fmtString string, args ...any) {
	if err != nil {
		args = append(args, err)
		e.errors = append(e.errors, fmt.Errorf(fmtString+": %w", args...))
	}
}

func (e *ErrorCollector) Addf(fmtString string, args ...any) {
	e.errors = append(e.errors, fmt.Errorf(fmtString, args...))
}

func (e *ErrorCollector) IsEmpty() bool {
	return e == nil || len(e.errors) == 0
}

func (e *ErrorCollector) Combined(prefix string) error {
	if e.IsEmpty() {
		return nil
	}
	if prefix == "" {
		if len(e.errors) == 1 {
			return e.errors[0]
		} else {
			return e
		}
	}
	if len(e.errors) == 1 {
		return fmt.Errorf("%s %w", prefix, e.errors[0])
	} else {
		if strings.Contains(e.Joiner, "\n") {
			return fmt.Errorf("%s %s%w", prefix, e.Joiner, e)
		} else {
			return fmt.Errorf("%s [%w]", prefix, e)
		}
	}
}

func CombineErrors(errors []error, joiner string) error {
	if len(errors) == 0 {
		return nil
	}
	return &ErrorCollector{errors: errors, Joiner: joiner}
}
