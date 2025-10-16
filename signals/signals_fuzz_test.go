package signals

import (
	"context"
	"os"
	"testing"
	"time"
)

// FuzzStateMachine exercises permutations of registry operations to shake out
// panics or bad state transitions. It avoids real OS signals.
func FuzzStateMachine(f *testing.F) {
	f.Add([]byte{0, 1, 2, 3, 4, 5, 6, 7, 8, 9})
	f.Add([]byte{4, 4, 0, 2, 3, 1, 5, 7, 8, 9})

	f.Fuzz(func(t *testing.T, data []byte) {
		r := NewRegistry()
		defer r.Stop()

		const maxOps = 256
		tokPool := make([]Token, 0, 32)

		for i := 0; i < len(data) && i < maxOps; i++ {
			op := data[i] % 10 // 0..9
			switch op {
			case 0: // Start
				_ = r.Start()
			case 1: // StartWithContext
				_ = r.StartWithContext(context.Background())
			case 2: // Stop
				r.Stop()
			case 3: // Reset
				r.Reset()
				tokPool = tokPool[:0]
			case 4: // Register
				toks := r.Register(func(ctx context.Context) {}, os.Interrupt)
				if len(toks) > 0 {
					tokPool = append(tokPool, toks[0])
				}
			case 5: // Unregister random token (or unknown token)
				if len(tokPool) > 0 {
					idx := int(data[i]) % len(tokPool)
					r.Unregister(tokPool[idx])
					tokPool = append(tokPool[:idx], tokPool[idx+1:]...)
				} else {
					// Ensure Unregister of unknown token is safe
					r.Unregister(Token{sig: os.Interrupt, id: 999})
				}
			case 6: // ContextFor
				ctx, cancel := r.ContextFor(context.Background(), os.Interrupt)
				select {
				case <-ctx.Done():
				default:
				}
				cancel()
			case 7: // beginShutdown
				r.beginShutdown()
			case 8: // incrementTermAndMaybeForce
				r.incrementTermAndMaybeForce()
			case 9: // force (idempotent)
				r.force()
			}
		}
	})
}

// FuzzPolicyShutdown probes grace periods and second-signal forcing.
func FuzzPolicyShutdown(f *testing.F) {
	f.Add(int64(0), true)  // zero grace with second signal
	f.Add(int64(5), false) // small grace without second signal

	f.Fuzz(func(t *testing.T, graceMs int64, second bool) {
		if graceMs < 0 {
			graceMs = 0
		}
		if graceMs > 1000 {
			graceMs = 1000
		}
		r := NewRegistry(WithPolicy(Policy{
			GracePeriod:         time.Duration(graceMs) * time.Millisecond,
			ForceOnSecondSignal: second,
			LogPanics:           (graceMs%2 == 0),
		}))
		defer r.Stop()

		ctx := r.handlerContext()
		r.beginShutdown()
		if second {
			r.incrementTermAndMaybeForce()
		}

		if graceMs == 0 && !second {
			select {
			case <-ctx.Done():
				t.Fatalf("unexpected force with zero grace and no second signal")
			case <-time.After(10 * time.Millisecond):
			}
        } else {
            // Allow for timer scheduling jitter: wait for grace + 150ms cushion.
            wait := time.Duration(graceMs)*time.Millisecond + 150*time.Millisecond
            if wait < 50*time.Millisecond {
                wait = 50 * time.Millisecond
            }
            select {
            case <-ctx.Done():
            case <-time.After(wait):
                t.Fatalf("expected cancellation; grace=%dms second=%v (waited %s)", graceMs, second, wait)
            }
        }
	})
}

// FuzzDefaultVsInstance mixes Default and an instance to ensure symmetry.
func FuzzDefaultVsInstance(f *testing.F) {
	f.Add([]byte{0, 1, 2, 3, 4, 5})

	f.Fuzz(func(t *testing.T, data []byte) {
		old := Default
		Default = NewRegistry()
		defer func() { Default.Stop(); Default = old }()

		r := NewRegistry()
		defer r.Stop()

		for i, b := range data {
			if i > 256 {
				break
			}
			targetDefault := (b & 1) == 1
			op := (b >> 1) % 6
			reg := r
			if targetDefault {
				reg = Default
			}
			switch op {
			case 0:
				reg.Register(func(ctx context.Context) {}, os.Interrupt)
    case 1:
        reg.Unregister(Token{sig: os.Interrupt, id: uint64(b)}) // unknown token: must be safe
			case 2:
				_ = reg.Start()
			case 3:
				reg.Stop()
			case 4:
				reg.Reset()
			case 5:
				_, cancel := reg.ContextFor(context.Background(), os.Interrupt)
				cancel()
			}
		}
	})
}
