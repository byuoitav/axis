package axis

import (
	"context"
	"fmt"
	"image"
	_ "image/jpeg"
	"mime"
	"mime/multipart"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

type P5414E struct {
	Address       string
	StreamProfile string
}

const (
	_p5414EEndpoint    = "/axis-cgi/com/ptz.cgi"
	_p5414EPanSpeed    = 5
	_p5414ETiltSpeed   = 5
	_p5414EZoomSpeed   = 25
	_p5414EPresetSpeed = 100

	_p5414ESnapshotEndpoint = "/axis-cgi/jpg/image.cgi"
	_p5414ESnapshotWidth    = 640
	_p5414ESnapshotHeight   = 360

	_p5414EStreamEndpoint = "/mjpg/video.mjpg"
)

func (c *P5414E) TiltUp(ctx context.Context) error {
	return c.PanTilt(ctx, 0, _p5414ETiltSpeed)
}

func (c *P5414E) TiltDown(ctx context.Context) error {
	return c.PanTilt(ctx, 0, -_p5414ETiltSpeed)
}

func (c *P5414E) PanLeft(ctx context.Context) error {
	return c.PanTilt(ctx, -_p5414EPanSpeed, 0)
}

func (c *P5414E) PanRight(ctx context.Context) error {
	return c.PanTilt(ctx, _p5414EPanSpeed, 0)
}

func (c *P5414E) PanTiltStop(ctx context.Context) error {
	return c.PanTilt(ctx, 0, 0)
}

func (c *P5414E) PanTilt(ctx context.Context, panSpeed int, tiltSpeed int) error {
	return c.do(ctx, url.Values{
		"continuouspantiltmove": []string{strconv.Itoa(panSpeed) + "," + strconv.Itoa(tiltSpeed)},
	})
}

func (c *P5414E) ZoomIn(ctx context.Context) error {
	return c.Zoom(ctx, _p5414EZoomSpeed)
}

func (c *P5414E) ZoomOut(ctx context.Context) error {
	return c.Zoom(ctx, -_p5414EZoomSpeed)
}

func (c *P5414E) ZoomStop(ctx context.Context) error {
	return c.Zoom(ctx, 0)
}

func (c *P5414E) Zoom(ctx context.Context, speed int) error {
	return c.do(ctx, url.Values{
		"continuouszoommove": []string{strconv.Itoa(speed)},
	})
}

func (c *P5414E) GoToPreset(ctx context.Context, preset string) error {
	return c.do(ctx, url.Values{
		"gotoserverpresetname": []string{preset},
		"speed":                []string{strconv.Itoa(_p5414EPresetSpeed)},
	})
}

func (c *P5414E) do(ctx context.Context, values url.Values) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fmt.Sprintf("http://%s%s", c.Address, _p5414EEndpoint), nil)
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

func (c *P5414E) Stream(ctx context.Context) (chan image.Image, chan error, error) {
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

	images := make(chan image.Image)
	errs := make(chan error)

	go func() {
		defer resp.Body.Close()

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

func (c *P5414E) Snapshot(ctx context.Context) (image.Image, error) {
	ctx, cancel := context.WithTimeout(ctx, 750*time.Millisecond)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fmt.Sprintf("http://%s%s", c.Address, _p5414ESnapshotEndpoint), nil)
	if err != nil {
		return nil, fmt.Errorf("unable to build request: %w", err)
	}

	req.URL.RawQuery = url.Values{
		"resolution": []string{strconv.Itoa(_p5414ESnapshotWidth) + "x" + strconv.Itoa(_p5414ESnapshotHeight)},
	}.Encode()

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("unable to make request: %w", err)
	}
	defer resp.Body.Close()

	image, _, err := image.Decode(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("unable to decode image: %w", err)
	}

	return image, nil
}
