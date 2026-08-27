package tekmetric

import (
	"fmt"

	"github.com/google/go-querystring/query"
)

// defaultPageSize is the page size the API uses when a caller sets none.
const defaultPageSize = 100

// encodeQuery builds a query string from the url tags on a params struct.
//
// The tags carry the wire names. A field tagged omitempty drops out when it
// holds a zero value, which matches the optional parameters the API accepts.
// A field without omitempty always encodes, which the API requires for page
// and size.
//
// A slice encodes as a repeated key, such as repairOrderStatusId=1 followed by
// repairOrderStatusId=2. The API reads a repeated key as a list.
func encodeQuery(params any) (string, error) {
	values, err := query.Values(params)
	if err != nil {
		return "", fmt.Errorf("failed to encode query parameters: %w", err)
	}
	return values.Encode(), nil
}

// normalizePageSize sets the page size to the default when a caller sets none.
// A negative size is not meaningful, so it takes the default too.
func normalizePageSize(size int) int {
	if size <= 0 {
		return defaultPageSize
	}
	return size
}
