// Package resource implements routines for managing resource acquisition and
// clean-up in Go code (also known as RAII).
package resource

type cleanupFunc func() error

type ResourceManager struct {
	cleanUps []cleanupFunc
}

func (r *ResourceManager) Acquire(acquire, release cleanupFunc) error {
	if r.cleanUps == nil {
		r.cleanUps = make([]cleanupFunc, 0)
	}
	err := acquire()
	if err != nil {
		return err
	}
	r.cleanUps = append(r.cleanUps, release)
	return nil
}

func (r *ResourceManager) Release() error {
	for i, j := 0, len(r.cleanUps)-1; i < j; i, j = i+1, j-1 {
		r.cleanUps[i], r.cleanUps[j] = r.cleanUps[j], r.cleanUps[i]
	}
	for _, cleanup := range r.cleanUps {
		err := cleanup()
		if err != nil {
			return err
		}
	}
	r.cleanUps = nil
	return nil
}
