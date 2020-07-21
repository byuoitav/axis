package axis

import (
	"context"
	"fmt"
	"image"
	_ "image/jpeg"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

type P5414E struct {
	Address string
}

const (
	_p5414EEndpoint    = "/axis-cgi/com/ptz.cgi"
	_p5414EPanSpeed    = 5
	_p5414ETiltSpeed   = 5
	_p5414EZoomSpeed   = 25
	_p5414EPresetSpeed = 100

	_p5414ESnapshotEndpoint = "/axis-cgi/jpg/image.cgi"
	_p5414ESnapshotWidth    = 1280
	_p5414ESnapshotHeight   = 720
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
	images := make(chan image.Image)
	errs := make(chan error)

	go func() {
		ticker := time.NewTicker(125 * time.Millisecond)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				image, err := c.getSnapshot(ctx)
				if err != nil {
					errs <- err
					continue
				}

				images <- image
			case <-ctx.Done():
				return
			}
		}
	}()

	return images, errs, nil
}

func (c *P5414E) getSnapshot(ctx context.Context) (image.Image, error) {
	ctx, cancel := context.WithTimeout(ctx, 500*time.Millisecond)
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
