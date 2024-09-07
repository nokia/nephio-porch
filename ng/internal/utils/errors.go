package utils

import (
	ctrl "sigs.k8s.io/controller-runtime"
)

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
