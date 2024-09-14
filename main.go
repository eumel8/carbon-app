package main

import (
	"context"
	"fmt"
	"image/color"
	"math"
	"os"
	"strconv"
	"time"

	"github.com/prometheus/client_golang/api"
	v1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/prometheus/common/model"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
)

const (
	defaultPullTime = 60
	nightStart      = 21
	nightEnd        = 6
	labelTextSize   = 120 // or 96
	ecoMetricLow    = 40
	ecoMetricHigh   = 80
	modeFullScreen  = true
)

type Config struct {
	prometheusURL string
	metricName    string
	pullPeriod    time.Duration
}

type myTheme struct{}

var _ fyne.Theme = (*myTheme)(nil)

// collect display size to set font size
// "github.com/kbinani/screenshot"
// bounds := screenshot.GetDisplayBounds(0)
// screenWidth := bounds.Dx()
// screenHeight := bounds.Dy()

func GetConfig() (Config, error) {
	pullTime, err := strconv.Atoi(os.Getenv("PULL_DURATION"))
	if err != nil {
		pullTime = defaultPullTime
	}
	return Config{
		prometheusURL: os.Getenv("PROMETHEUS_URL"),
		metricName:    "entsoe_generation_eco",
		pullPeriod:    time.Duration(pullTime) * time.Second,
	}, nil
}

func (m myTheme) Font(style fyne.TextStyle) fyne.Resource {
	return theme.DefaultTheme().Font(style)
}

func (m myTheme) Size(name fyne.ThemeSizeName) float32 {
	return theme.DefaultTheme().Size(name)
}

func (m myTheme) Color(name fyne.ThemeColorName, variant fyne.ThemeVariant) color.Color {
	c, err := GetConfig()
	if err != nil {
		fmt.Printf("Error getting config: %v\n", err)
		return color.Black
	}
	carbonColor, err := c.CarbonColor()
	if err != nil {
		fmt.Printf("Error getting carbon color: %v\n", err)
		return color.Black
	}
	return carbonColor
}

func (m myTheme) Icon(name fyne.ThemeIconName) fyne.Resource {
	return theme.DefaultTheme().Icon(name)
}

func (c *Config) GetCarbonMetric() (int, error) {
	if c.prometheusURL == "" {
		return 0, fmt.Errorf("PROMETHEUS_URL environment variable is not set")
	}

	client, err := api.NewClient(api.Config{
		Address: c.prometheusURL,
	})
	if err != nil {
		fmt.Printf("Error creating client: %v\n", err)
		return 0, err
	}

	v1api := v1.NewAPI(client)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	result, _, err := v1api.Query(ctx, c.metricName, time.Now())
	if err != nil {
		fmt.Printf("Error querying Prometheus: %v\n", err)
		return 0, err
	}

	vectorVal, ok := result.(model.Vector)
	if !ok || len(vectorVal) == 0 {
		return 0, fmt.Errorf("no data returned")
	}
	carbonMetric := vectorVal[0].Value * 100
	fmt.Println("carbonMetric: ", carbonMetric)

	// Round the metric to two decimal places
	roundedMetric := math.Round(float64(carbonMetric)*100) / 100

	formatMetric, err := strconv.Atoi(fmt.Sprintf("%.0f", roundedMetric))
	if err != nil {
		fmt.Printf("Error formatting metric: %v\n", err)
		return 0, err
	}
	return formatMetric, nil
}

func (c *Config) CarbonColor() (color.Color, error) {

	// default color grey
	carbonColor := color.RGBA{125, 125, 125, 255}
	carbonMetric, err := c.GetCarbonMetric()

	if err != nil {
		fmt.Printf("Error querying Prometheus: %v\n", err)
		return color.RGBA{}, err
	}

	if carbonMetric <= ecoMetricLow && carbonMetric > 0 {
		if isNight() {
			// dark red/brown
			carbonColor = color.RGBA{140, 0, 0, 255}
		} else {
			// red
			carbonColor = color.RGBA{255, 0, 0, 255}
		}
	} else if carbonMetric > ecoMetricLow && carbonMetric <= ecoMetricHigh {
		if isNight() {
			// dark yellow
			carbonColor = color.RGBA{175, 175, 0, 200}
		} else {
			// light yellow
			carbonColor = color.RGBA{255, 255, 0, 255}
		}
	} else {
		if isNight() {
			// dark green
			carbonColor = color.RGBA{0, 190, 0, 255}
		} else {
			// light green
			carbonColor = color.RGBA{0, 255, 0, 255}
		}
	}
	return carbonColor, nil
}

// find out if it is night to dim the display
func isNight() bool {
	now := time.Now()
	hour := now.Hour()
	return hour >= nightStart || hour < nightEnd
}

func main() {
	c, err := GetConfig()
	if err != nil {
		fmt.Printf("Error reading config: %v\n", err)
		return
	}
	iconResource, err := fyne.LoadResourceFromURLString("https://raw.githubusercontent.com/eumel8/carbon-app/main/icon.png")
	if err != nil {
		fmt.Printf("Failed to load icon", err)
		return
	}

	carbonApp := app.New()
	carbonApp.SetIcon(iconResource)
	carbonWindow := carbonApp.NewWindow("Carbon-App")
	carbonWindow.SetFullScreen(modeFullScreen)

	mainLabel := canvas.NewText("Show the current carbon emission", color.White)
	mainContent := container.NewVBox(mainLabel)

	go func() {
		for {
			carbonApp.Settings().SetTheme(&myTheme{})
			carbonMetric, err := c.GetCarbonMetric()
			if err != nil {
				fmt.Printf("Error querying Prometheus: %v\n", err)
			}
			currentTime := time.Now().Format("02.01.2006 15:04:05")
			timeLabel := canvas.NewText(currentTime, color.Gray{})
			timeLabel.Alignment = fyne.TextAlignCenter
			carbonLabel := canvas.NewText(fmt.Sprintf("%d ", carbonMetric), color.Black)
			carbonLabel.TextStyle.Bold = true
			carbonLabel.TextSize = labelTextSize
			carbonLabel.Alignment = fyne.TextAlignCenter
			content := container.NewVBox(timeLabel, carbonLabel)
			carbonLabel.Refresh()
			timeLabel.Refresh()
			carbonWindow.SetContent(content)
			carbonWindow.Canvas().Refresh(content)
			carbonWindow.Canvas().SetOnTypedKey(func(keyEvent *fyne.KeyEvent) {
				if keyEvent.Name == fyne.KeyEscape {
					carbonApp.Quit()
				}
			})
			time.Sleep(c.pullPeriod)
		}
	}()
	carbonWindow.SetContent(mainContent)
	carbonWindow.ShowAndRun()
}
