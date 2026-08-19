/*
Copyright 2025 Ross Golder
*/

package clients

import (
	"fmt"

	"testing"
	"time"

	"github.com/rossigee/provider-libvirt/apis/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestGetBackoffDuration(t *testing.T) {
	tests := []struct {
		name         string
		failureCount int
		expectedMin  time.Duration
		expectedMax  time.Duration
	}{
		{
			name:         "zero failures",
			failureCount: 0,
			expectedMin:  5 * time.Second,
			expectedMax:  5 * time.Second,
		},
		{
			name:         "one failure",
			failureCount: 1,
			expectedMin:  10 * time.Second,
			expectedMax:  10 * time.Second,
		},
		{
			name:         "two failures",
			failureCount: 2,
			expectedMin:  20 * time.Second,
			expectedMax:  20 * time.Second,
		},
		{
			name:         "three failures",
			failureCount: 3,
			expectedMin:  40 * time.Second,
			expectedMax:  40 * time.Second,
		},
		{
			name:         "excessive failures (capped at max)",
			failureCount: 100,
			expectedMin:  5 * time.Minute,
			expectedMax:  5 * time.Minute,
		},
		{
			name:         "ten failures (should hit cap)",
			failureCount: 10,
			expectedMin:  5 * time.Minute,
			expectedMax:  5 * time.Minute,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := GetBackoffDuration(tt.failureCount)
			if got < tt.expectedMin || got > tt.expectedMax {
				t.Errorf("GetBackoffDuration(%d) = %v, want between %v and %v",
					tt.failureCount, got, tt.expectedMin, tt.expectedMax)
			}
		})
	}
}

func TestIsTransientError(t *testing.T) {
	tests := []struct {
		name          string
		err           error
		shouldBeRetry bool
	}{
		{
			name:          "nil error",
			err:           nil,
			shouldBeRetry: false,
		},
		{
			name:          "connection refused",
			err:           fmt.Errorf("connection refused"),
			shouldBeRetry: true,
		},
		{
			name:          "connection reset",
			err:           fmt.Errorf("connection reset by peer"),
			shouldBeRetry: true,
		},
		{
			name:          "timeout",
			err:           fmt.Errorf("i/o timeout"),
			shouldBeRetry: true,
		},
		{
			name:          "CA certificate error",
			err:           fmt.Errorf("certificate verification failed: no CA certificate"),
			shouldBeRetry: true,
		},
		{
			name:          "TLS error",
			err:           fmt.Errorf("TLS handshake failed"),
			shouldBeRetry: true,
		},
		{
			name:          "authentication error",
			err:           fmt.Errorf("authentication failed"),
			shouldBeRetry: true,
		},
		{
			name:          "permission denied",
			err:           fmt.Errorf("permission denied: socket access denied"),
			shouldBeRetry: true,
		},
		{
			name:          "DNS failure",
			err:           fmt.Errorf("name or service not known"),
			shouldBeRetry: true,
		},
		{
			name:          "temporary failure",
			err:           fmt.Errorf("temporary failure in name resolution"),
			shouldBeRetry: true,
		},
		{
			name:          "permanent error",
			err:           fmt.Errorf("invalid URI format"),
			shouldBeRetry: false,
		},
		{
			name:          "bad credentials",
			err:           fmt.Errorf("invalid username/password"),
			shouldBeRetry: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsTransientError(tt.err)
			if got != tt.shouldBeRetry {
				t.Errorf("IsTransientError(%v) = %v, want %v",
					tt.err, got, tt.shouldBeRetry)
			}
		})
	}
}

func TestGetConnectionFailureCount(t *testing.T) {
	tests := []struct {
		name          string
		pc            *v1beta1.ProviderConfig
		expectedCount int
	}{
		{
			name: "no annotations",
			pc: &v1beta1.ProviderConfig{
				ObjectMeta: metav1.ObjectMeta{Name: "test"},
			},
			expectedCount: 0,
		},
		{
			name: "empty annotations",
			pc: &v1beta1.ProviderConfig{
				ObjectMeta: metav1.ObjectMeta{
					Name:        "test",
					Annotations: map[string]string{},
				},
			},
			expectedCount: 0,
		},
		{
			name: "count annotation present",
			pc: &v1beta1.ProviderConfig{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test",
					Annotations: map[string]string{
						AnnotationConnFailureCount: "5",
					},
				},
			},
			expectedCount: 5,
		},
		{
			name: "zero count annotation",
			pc: &v1beta1.ProviderConfig{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test",
					Annotations: map[string]string{
						AnnotationConnFailureCount: "0",
					},
				},
			},
			expectedCount: 0,
		},
		{
			name: "invalid count annotation (defaults to 0)",
			pc: &v1beta1.ProviderConfig{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test",
					Annotations: map[string]string{
						AnnotationConnFailureCount: "not-a-number",
					},
				},
			},
			expectedCount: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := getConnectionFailureCount(tt.pc)
			if got != tt.expectedCount {
				t.Errorf("getConnectionFailureCount() = %d, want %d",
					got, tt.expectedCount)
			}
		})
	}
}

func TestShouldBackoffConnection(t *testing.T) {
	now := time.Now()
	nowRFC3339 := now.Format(time.RFC3339)

	tests := []struct {
		name          string
		pc            *v1beta1.ProviderConfig
		shouldBackoff bool
	}{
		{
			name: "no annotations",
			pc: &v1beta1.ProviderConfig{
				ObjectMeta: metav1.ObjectMeta{Name: "test"},
			},
			shouldBackoff: false,
		},
		{
			name: "no failure time annotation",
			pc: &v1beta1.ProviderConfig{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test",
					Annotations: map[string]string{
						AnnotationConnFailureCount: "5",
					},
				},
			},
			shouldBackoff: false,
		},
		{
			name: "recent failure (should backoff)",
			pc: &v1beta1.ProviderConfig{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test",
					Annotations: map[string]string{
						AnnotationLastConnFailureTime: nowRFC3339,
						AnnotationConnFailureCount:    "1",
					},
				},
			},
			shouldBackoff: true,
		},
		{
			name: "old failure (should not backoff)",
			pc: &v1beta1.ProviderConfig{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test",
					Annotations: map[string]string{
						AnnotationLastConnFailureTime: now.Add(-10 * time.Minute).Format(time.RFC3339),
						AnnotationConnFailureCount:    "1",
					},
				},
			},
			shouldBackoff: false,
		},
		{
			name: "invalid time format",
			pc: &v1beta1.ProviderConfig{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test",
					Annotations: map[string]string{
						AnnotationLastConnFailureTime: "invalid-time",
						AnnotationConnFailureCount:    "5",
					},
				},
			},
			shouldBackoff: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := shouldBackoffConnection(tt.pc)
			if got != tt.shouldBackoff {
				t.Errorf("shouldBackoffConnection() = %v, want %v",
					got, tt.shouldBackoff)
			}
		})
	}
}

func TestRetriableError(t *testing.T) {
	innerErr := fmt.Errorf("connection refused")
	backoff := 30 * time.Second
	retriableErr := &RetriableError{
		Err:       innerErr,
		Backoff:   backoff,
		Retriable: true,
	}

	// Test Error() method
	if retriableErr.Error() != "connection refused" {
		t.Errorf("RetriableError.Error() = %s, want 'connection refused'",
			retriableErr.Error())
	}

	// Test Unwrap() method
	if retriableErr.Unwrap() != innerErr {
		t.Errorf("RetriableError.Unwrap() didn't return original error")
	}

	// Test ExtractRetriableBackoff
	duration, ok := ExtractRetriableBackoff(retriableErr)
	if !ok {
		t.Errorf("ExtractRetriableBackoff() returned ok=false, want true")
	}
	if duration != backoff {
		t.Errorf("ExtractRetriableBackoff() = %v, want %v",
			duration, backoff)
	}

	// Test with nil error
	_, ok = ExtractRetriableBackoff(nil)
	if ok {
		t.Errorf("ExtractRetriableBackoff(nil) returned ok=true, want false")
	}

	// Test with non-retriable error
	_, ok = ExtractRetriableBackoff(fmt.Errorf("some error"))
	if ok {
		t.Errorf("ExtractRetriableBackoff(non-retriable) returned ok=true, want false")
	}
}
