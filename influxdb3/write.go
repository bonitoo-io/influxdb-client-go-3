/*
 The MIT License

 Permission is hereby granted, free of charge, to any person obtaining a copy
 of this software and associated documentation files (the "Software"), to deal
 in the Software without restriction, including without limitation the rights
 to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
 copies of the Software, and to permit persons to whom the Software is
 furnished to do so, subject to the following conditions:

 The above copyright notice and this permission notice shall be included in
 all copies or substantial portions of the Software.

 THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
 IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
 FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
 AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
 LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
 OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
 THE SOFTWARE.
*/

package influxdb3

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"reflect"
	"strings"
	"time"

	"github.com/InfluxCommunity/influxdb3-go/influxdb3/gzip"
	"github.com/influxdata/line-protocol/v2/lineprotocol"
)

// WritePoints writes all the given points to the server into the given database.
// The data is written synchronously.
//
// Parameters:
//   - ctx: The context.Context to use for the request.
//   - points: The points to write.
//   - options: Optional write options. See WriteOption for available options.
//
// Returns:
//   - An error, if any.
func (c *Client) WritePoints(ctx context.Context, points []*Point, options ...WriteOption) error {
	return c.writePoints(ctx, points, newWriteOptions(c.config.WriteOptions, options))
}

// WritePointsWithOptions writes all the given points to the server into the given database.
// The data is written synchronously.
//
// Parameters:
//   - ctx: The context.Context to use for the request.
//   - points: The points to write.
//   - options: Write options.
//
// Returns:
//   - An error, if any.
//
// Deprecated: use WritePoints with variadic WriteOption options.
func (c *Client) WritePointsWithOptions(ctx context.Context, options *WriteOptions, points ...*Point) error {
	if options == nil {
		return errors.New("options not set")
	}

	return c.writePoints(ctx, points, options)
}

func (c *Client) writePoints(ctx context.Context, points []*Point, options *WriteOptions) error {
	var buff []byte
	var precision lineprotocol.Precision
	if options != nil {
		precision = options.Precision
	} else {
		precision = c.config.WriteOptions.Precision
	}
	var defaultTags map[string]string
	if options != nil && options.DefaultTags != nil {
		defaultTags = options.DefaultTags
	} else {
		defaultTags = c.config.WriteOptions.DefaultTags
	}

	for _, p := range points {
		bts, err := p.MarshalBinaryWithDefaultTags(precision, defaultTags)
		if err != nil {
			return err
		}
		buff = append(buff, bts...)
	}

	return c.write(ctx, buff, options)
}

// Write writes line protocol record(s) to the server into the given database.
// Multiple records must be separated by the new line character (\n).
// The data is written synchronously.
//
// Parameters:
//   - ctx: The context.Context to use for the request.
//   - buff: The line protocol record(s) to write.
//   - options: Optional write options. See WriteOption for available options.
//
// Returns:
//   - An error, if any.
func (c *Client) Write(ctx context.Context, buff []byte, options ...WriteOption) error {
	return c.write(ctx, buff, newWriteOptions(c.config.WriteOptions, options))
}

// WriteWithOptions writes line protocol record(s) to the server into the given database.
// Multiple records must be separated by the new line character (\n).
// The data is written synchronously.
//
// Parameters:
//   - ctx: The context.Context to use for the request.
//   - buff: The line protocol record(s) to write.
//   - options: Write options.
//
// Returns:
//   - An error, if any.
//
// Deprecated: use Write with variadic WriteOption option
func (c *Client) WriteWithOptions(ctx context.Context, options *WriteOptions, buff []byte) error {
	if options == nil {
		return errors.New("options not set")
	}

	return c.write(ctx, buff, options)
}

func (c *Client) write(ctx context.Context, buff []byte, options *WriteOptions) error {
	var database string
	if options.Database != "" {
		database = options.Database
	} else {
		database = c.config.Database
	}
	if database == "" {
		return errors.New("database not specified")
	}

	var precision = options.Precision

	var gzipThreshold = options.GzipThreshold

	var body io.Reader
	var err error
	u, _ := c.apiURL.Parse("write")
	params := u.Query()
	params.Set("org", c.config.Organization)
	params.Set("bucket", database)
	params.Set("precision", precision.String())
	u.RawQuery = params.Encode()
	body = bytes.NewReader(buff)
	headers := http.Header{"Content-Type": {"application/json"}}
	if gzipThreshold > 0 && len(buff) >= gzipThreshold {
		body, err = gzip.CompressWithGzip(body)
		if err != nil {
			return fmt.Errorf("unable to compress write body: %w", err)
		}
		headers["Content-Encoding"] = []string{"gzip"}
	}
	resp, err := c.makeAPICall(ctx, httpParams{
		endpointURL: u,
		httpMethod:  "POST",
		headers:     headers,
		queryParams: u.Query(),
		body:        body,
	})
	if err != nil {
		return err
	}
	return resp.Body.Close()
}

// WriteData encodes fields of custom points into line protocol
// and writes line protocol record(s) to the server into the given database.
// Each custom point must be annotated with 'lp' prefix and Values measurement, tag, field, or timestamp.
// A valid point must contain a measurement and at least one field.
// The points are written synchronously.
//
// A field with a timestamp must be of type time.Time.
//
// Parameters:
//   - ctx: The context.Context to use for the request.
//   - points: The custom points to encode and write.
//   - options: Optional write options. See WriteOption for available options.
//
// Returns:
//   - An error, if any.
func (c *Client) WriteData(ctx context.Context, points []interface{}, options ...WriteOption) error {
	return c.writeData(ctx, points, newWriteOptions(c.config.WriteOptions, options))
}

// WriteDataWithOptions encodes fields of custom points into line protocol
// and writes line protocol record(s) to the server into the given database.
// Each custom point must be annotated with 'lp' prefix and Values measurement, tag, field, or timestamp.
// A valid point must contain a measurement and at least one field.
// The points are written synchronously.
//
// A field with a timestamp must be of type time.Time.
//
// Parameters:
//   - ctx: The context.Context to use for the request.
//   - points: The custom points to encode and write.
//   - options: Write options.
//
// Returns:
//   - An error, if any.
//
// Deprecated: use WriteData with variadic WriteOption option
func (c *Client) WriteDataWithOptions(ctx context.Context, options *WriteOptions, points ...interface{}) error {
	if options == nil {
		return errors.New("options not set")
	}

	return c.writeData(ctx, points, options)
}

func (c *Client) writeData(ctx context.Context, points []interface{}, options *WriteOptions) error {
	var buff []byte
	for _, p := range points {
		b, err := encode(p, options)
		if err != nil {
			return fmt.Errorf("error encoding point: %w", err)
		}
		buff = append(buff, b...)
	}

	return c.write(ctx, buff, options)
}

func encode(x interface{}, options *WriteOptions) ([]byte, error) {
	if err := checkContainerType(x, false, "point"); err != nil {
		return nil, err
	}

	t := reflect.TypeOf(x)
	v := reflect.ValueOf(x)
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
		v = v.Elem()
	}
	fields := reflect.VisibleFields(t)

	var point = &Point{
		Values: &PointValues{
			Tags:   make(map[string]string),
			Fields: make(map[string]interface{}),
		},
	}

	for _, f := range fields {
		name := f.Name
		if tag, ok := f.Tag.Lookup("lp"); ok {
			if tag == "-" {
				continue
			}
			parts := strings.Split(tag, ",")
			if len(parts) > 2 {
				return nil, errors.New("multiple tag attributes are not supported")
			}
			typ := parts[0]
			if len(parts) == 2 {
				name = parts[1]
			}
			switch typ {
			case "measurement":
				if point.GetMeasurement() != "" {
					return nil, errors.New("multiple measurement fields")
				}
				point.SetMeasurement(v.FieldByIndex(f.Index).String())
			case "tag":
				point.SetTag(name, v.FieldByIndex(f.Index).String())
			case "field":
				point.SetField(name, v.FieldByIndex(f.Index).Interface())
			case "timestamp":
				if f.Type != timeType {
					return nil, fmt.Errorf("cannot use field '%s' as a timestamp", f.Name)
				}
				point.SetTimestamp(v.FieldByIndex(f.Index).Interface().(time.Time))
			default:
				return nil, fmt.Errorf("invalid tag %s", typ)
			}
		}
	}
	if point.GetMeasurement() == "" {
		return nil, errors.New("no struct field with tag 'measurement'")
	}
	if !point.HasFields() {
		return nil, errors.New("no struct field with tag 'field'")
	}

	return point.MarshalBinaryWithDefaultTags(options.Precision, options.DefaultTags)
}
