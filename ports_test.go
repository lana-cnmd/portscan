package portscan

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPorts(t *testing.T) {
	tt := []struct {
		name          string
		ports         []uint16
		expectedPorts []uint16
		expectedErr   error
	}{
		{
			name:          "Success",
			ports:         []uint16{1, 3, 80},
			expectedPorts: []uint16{1, 3, 80},
			expectedErr:   nil,
		},
		{
			name:        "Invalid port error",
			ports:       []uint16{0, 3, 80},
			expectedErr: &InvalidPortError{Port: 0},
		},
		{
			name:        "Empty port",
			ports:       []uint16{},
			expectedErr: nil,
		},
		{
			name:          "Duplicated ports",
			ports:         []uint16{80, 80, 443},
			expectedPorts: []uint16{80, 80, 443},
			expectedErr:   nil,
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			res, err := Ports(tc.ports...)
			if tc.expectedErr != nil {
				var invalidPortErr *InvalidPortError
				require.ErrorAs(t, err, &invalidPortErr)
				assert.Equal(t, uint16(0), invalidPortErr.Port)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tc.expectedPorts, res.ports)
			}
		})
	}
}

func TestRange(t *testing.T) {
	tt := []struct {
		name          string
		start         uint16
		end           uint16
		expectedPorts []uint16
		expectedErr   error
	}{
		{
			name:          "Success",
			start:         1,
			end:           8,
			expectedPorts: []uint16{1, 2, 3, 4, 5, 6, 7, 8},
			expectedErr:   nil,
		},
		{
			name:        "Full range 1-65535",
			start:       1,
			end:         65535,
			expectedErr: nil,
		},
		{
			name:          "Invalid start port error",
			start:         0,
			end:           100,
			expectedPorts: []uint16{},
			expectedErr:   &InvalidPortError{Port: 0},
		},
		{
			name:          "Invalid end port error",
			start:         100,
			end:           0,
			expectedPorts: []uint16{},
			expectedErr:   &InvalidPortError{Port: 0},
		},
		{
			name:          "Invalid range error",
			start:         100,
			end:           1,
			expectedPorts: []uint16{},
			expectedErr:   &InvalidRangeError{Start: 100, End: 1},
		},
		{
			name:          "duplicated start and end values",
			start:         1,
			end:           1,
			expectedPorts: []uint16{1},
			expectedErr:   nil,
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			res, err := Range(tc.start, tc.end)
			switch expected := tc.expectedErr.(type) {
			case nil:
				require.NoError(t, err)
				assert.Equal(t, tc.expectedPorts, res.ports)
			case *InvalidPortError:
				var actual *InvalidPortError
				require.ErrorAs(t, err, &actual)
				assert.Equal(t, expected.Port, actual.Port)
			case *InvalidRangeError:
				var actual *InvalidRangeError
				require.ErrorAs(t, err, &actual)
				assert.Equal(t, expected.Start, actual.Start)
				assert.Equal(t, expected.End, actual.End)
			default:
				t.Fatalf("unexpected expectedErr type: %T", tc.expectedErr)
			}

		})
	}
}
