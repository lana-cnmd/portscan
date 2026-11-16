package portscan

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNormalizeHosts(t *testing.T) {
	tt := []struct {
		name          string
		hosts         []string
		expectedHosts []string
		expectedErr   error
	}{
		{
			name:          "Success",
			hosts:         []string{"1.1.1.1", "google.com"},
			expectedHosts: []string{"1.1.1.1", "google.com"},
			expectedErr:   nil,
		},
		{
			name:          "Deduplication",
			hosts:         []string{"1.1.1.1", "1.1.1.1"},
			expectedHosts: []string{"1.1.1.1"},
			expectedErr:   nil,
		},
		{
			name:          "Empty list",
			hosts:         []string{},
			expectedHosts: nil,
			expectedErr:   ErrNoTargets,
		},
		{
			name:          "Nil list",
			hosts:         nil,
			expectedHosts: nil,
			expectedErr:   ErrNoTargets,
		},
		{
			name:          "Empty host",
			hosts:         []string{"1.1.1.1", ""},
			expectedHosts: nil,
			expectedErr:   ErrEmptyHost,
		},
		{
			name:          "Empty host only",
			hosts:         []string{""},
			expectedHosts: nil,
			expectedErr:   ErrEmptyHost,
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			res, err := normalizeHosts(tc.hosts)
			if tc.expectedErr != nil {
				require.ErrorIs(t, err, tc.expectedErr)
				assert.Nil(t, res)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tc.expectedHosts, res)
			}
		})
	}
}
