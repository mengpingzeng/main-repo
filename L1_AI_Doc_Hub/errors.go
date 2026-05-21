package a4md

import "errors"

var (
	ErrTaskAlreadyExists  = errors.New("a4md: task MD already exists, use AppendStats instead")
	ErrWriteMDNotExist    = errors.New("a4md: task MD not found, must call WriteMD before AppendStats")
	ErrSectionDuplicate   = errors.New("a4md: section already exists for this period, append is idempotent")
	ErrTemplateRenderFail = errors.New("a4md: template rendering failed")
	ErrOSSWriteFail       = errors.New("a4md: OSS write failed")
	ErrOSSReadFail        = errors.New("a4md: OSS read failed")
	ErrFileTooLarge       = errors.New("a4md: file exceeds 5MB limit, must split")
)

func wrapErr(target, src error) error {
	return &wrappedError{target: target, src: src}
}

type wrappedError struct {
	target error
	src    error
}

func (e *wrappedError) Error() string { return e.target.Error() + ": " + e.src.Error() }
func (e *wrappedError) Unwrap() error { return e.src }
func (e *wrappedError) Is(target error) bool { return errors.Is(e.target, target) || errors.Is(e.src, target) }

func fmtAppendErr(msg string, err error) error {
	return &wrappedError{
		target: ErrWriteMDNotExist,
		src:    errors.New(msg + ": " + err.Error()),
	}
}
