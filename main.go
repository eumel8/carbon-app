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
	// "fyne.io/fyne/v2/widget"
)

type Config struct {
	prometheusURL string
	metricName    string
	pullPeriod    time.Duration
}

type myTheme struct{}

var _ fyne.Theme = (*myTheme)(nil)

func GetConfig() (Config, error) {
	pullTime, err := strconv.Atoi(os.Getenv("PULL_DURATION"))
	if err != nil {
		pullTime = 60
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

	carbonColor := color.RGBA{0, 255, 0, 255}
	carbonMetric, err := c.GetCarbonMetric()

	if err != nil {
		fmt.Printf("Error querying Prometheus: %v\n", err)
		return color.RGBA{}, err
	}

	if carbonMetric <= 40 {
		carbonColor = color.RGBA{255, 0, 0, 255}
	} else if carbonMetric > 40 && carbonMetric <= 80 {
		carbonColor = color.RGBA{255, 255, 0, 255}
	}

	return carbonColor, nil
}

func main() {

	c, err := GetConfig()
	if err != nil {
		fmt.Printf("Error reading config: %v\n", err)
		return
	}
	carbonApp := app.New()
	carbonWindow := carbonApp.NewWindow("Carbon-App")
	carbonWindow.SetFullScreen(true)

	mainLabel := canvas.NewText("Show the current carbon emission", color.White)
	mainContent := container.NewVBox(mainLabel)

	go func() {
		for {
			time.Sleep(c.pullPeriod)
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
			carbonLabel.TextSize = 72
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
		}
	}()
	carbonWindow.SetContent(mainContent)
	carbonWindow.ShowAndRun()
}
