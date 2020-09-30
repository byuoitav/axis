package axis

import (
	"context"
	"fmt"
	"image"
	_ "image/jpeg"
	"io/ioutil"
	"mime"
	"mime/multipart"
	"net/http"
	"net/url"
	"strconv"
)

type V5915 struct {
	Address       string
	StreamProfile string
}

const (
	_v5915Endpoint    = "/axis-cgi/com/ptz.cgi"
	_v5915PanSpeed    = 50
	_v5915TiltSpeed   = 40
	_v5915ZoomSpeed   = 75
	_v5915PresetSpeed = 100

	_v5915StreamEndpoint = "/mjpg/video.mjpg"
)

func (c *V5915) RemoteAddr() string {
	return c.Address
}

func (c *V5915) TiltUp(ctx context.Context) error {
	return c.PanTilt(ctx, 0, _v5915TiltSpeed)
}

func (c *V5915) TiltDown(ctx context.Context) error {
	return c.PanTilt(ctx, 0, -_v5915TiltSpeed)
}

func (c *V5915) PanLeft(ctx context.Context) error {
	return c.PanTilt(ctx, -_v5915PanSpeed, 0)
}

func (c *V5915) PanRight(ctx context.Context) error {
	return c.PanTilt(ctx, _v5915PanSpeed, 0)
}

func (c *V5915) PanTiltStop(ctx context.Context) error {
	return c.PanTilt(ctx, 0, 0)
}

func (c *V5915) PanTilt(ctx context.Context, panSpeed int, tiltSpeed int) error {
	return c.do(ctx, url.Values{
		"continuouspantiltmove": []string{strconv.Itoa(panSpeed) + "," + strconv.Itoa(tiltSpeed)},
	})
}

func (c *V5915) ZoomIn(ctx context.Context) error {
	return c.Zoom(ctx, _v5915ZoomSpeed)
}

func (c *V5915) ZoomOut(ctx context.Context) error {
	return c.Zoom(ctx, -_v5915ZoomSpeed)
}

func (c *V5915) ZoomStop(ctx context.Context) error {
	return c.Zoom(ctx, 0)
}

func (c *V5915) Zoom(ctx context.Context, speed int) error {
	return c.do(ctx, url.Values{
		"continuouszoommove": []string{strconv.Itoa(speed)},
	})
}

func (c *V5915) GoToPreset(ctx context.Context, preset string) error {
	return c.do(ctx, url.Values{
		"gotoserverpresetname": []string{preset},
		"speed":                []string{strconv.Itoa(_v5915PresetSpeed)},
	})
}

func (c *V5915) do(ctx context.Context, values url.Values) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fmt.Sprintf("http://%s%s", c.Address, _v5915Endpoint), nil)
	if err != nil {
		return fmt.Errorf("unable to build request: %w", err)
	}

	req.URL.RawQuery = values.Encode()

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("unable to make request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode/100 != 2 {
		return fmt.Errorf("request failed: %d response from camera", resp.StatusCode)
	}

	return nil
}

func (c *V5915) StreamJPEG(ctx context.Context) (chan []byte, chan error, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fmt.Sprintf("http://%s%s", c.Address, _p5414EStreamEndpoint), nil)
	if err != nil {
		return nil, nil, fmt.Errorf("unable to build request: %w", err)
	}

	if c.StreamProfile != "" {
		req.URL.RawQuery = url.Values{
			"streamprofile": []string{c.StreamProfile},
		}.Encode()
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, nil, fmt.Errorf("unable to make request: %w", err)
	}

	_, params, err := mime.ParseMediaType(resp.Header.Get("Content-Type"))
	if err != nil {
		resp.Body.Close()
		return nil, nil, fmt.Errorf("unable to parse content-type: %w", err)
	}

	jpegs := make(chan []byte)
	errs := make(chan error)

	go func() {
		defer resp.Body.Close()
		defer close(jpegs)
		defer close(errs)

		reader := multipart.NewReader(resp.Body, params["boundary"])

		for {
			select {
			case <-ctx.Done():
				return
			default:
				part, err := reader.NextPart()
				if err != nil {
					errs <- fmt.Errorf("unable to read next frame: %w", err)
					continue
				}

				jpeg, err := ioutil.ReadAll(part)
				if err != nil {
					errs <- fmt.Errorf("unable to read part: %w", err)
					continue
				}

				jpegs <- jpeg
			}
		}

	}()

	return jpegs, errs, nil
}

func (c *V5915) Stream(ctx context.Context) (chan image.Image, chan error, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fmt.Sprintf("http://%s%s", c.Address, _v5915StreamEndpoint), nil)
	if err != nil {
		return nil, nil, fmt.Errorf("unable to build request: %w", err)
	}

	if c.StreamProfile != "" {
		req.URL.RawQuery = url.Values{
			"streamprofile": []string{c.StreamProfile},
		}.Encode()
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, nil, fmt.Errorf("unable to make request: %w", err)
	}

	_, params, err := mime.ParseMediaType(resp.Header.Get("Content-Type"))
	if err != nil {
		resp.Body.Close()
		return nil, nil, fmt.Errorf("unable to parse content-type: %w", err)
	}

	images := make(chan image.Image)
	errs := make(chan error)

	go func() {
		defer resp.Body.Close()
		defer close(images)
		defer close(errs)

		reader := multipart.NewReader(resp.Body, params["boundary"])

		for {
			select {
			case <-ctx.Done():
				return
			default:
				part, err := reader.NextPart()
				if err != nil {
					errs <- fmt.Errorf("unable to read next frame: %w", err)
					continue
				}

				image, _, err := image.Decode(part)
				if err != nil {
					errs <- fmt.Errorf("unable to decode image: %w", err)
					continue
				}

				images <- image
			}
		}

	}()

	return images, errs, nil
}
