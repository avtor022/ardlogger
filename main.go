package main

import (
	"bufio"
	"fmt"
	"image/color"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	"github.com/jacobsa/go-serial/serial"
)

// RFIDLogger хранит состояние приложения
type RFIDLogger struct {
	port       serial.Port
	isConnected bool
	logText    *widget.RichText
	logScroll  *container.Scroll
	statusText *widget.RichText
	connectBtn *widget.Button
	portEntry  *widget.Entry
	baudEntry  *widget.Entry
}

func NewRFIDLogger() *RFIDLogger {
	return &RFIDLogger{
		isConnected: false,
	}
}

func (r *RFIDLogger) connect() {
	if r.isConnected {
		r.disconnect()
		return
	}

	portName := r.portEntry.Text
	if portName == "" {
		portName = "/dev/ttyUSB0" // значение по умолчанию для Linux
	}

	baudRate := uint(9600)
	if r.baudEntry.Text != "" {
		fmt.Sscanf(r.baudEntry.Text, "%d", &baudRate)
	}

	options := serial.OpenOptions{
		PortName:        portName,
		BaudRate:        baudRate,
		DataBits:        8,
		StopBits:        1,
		MinimumReadSize: 4,
	}

	port, err := serial.Open(options)
	if err != nil {
		r.statusText.Segments = []widget.RichTextSegment{
			&widget.TextSegment{
				Text: fmt.Sprintf("Ошибка подключения: %v", err),
				Style: widget.RichTextStyle{
					Color: theme.ColorRed,
					Inline: true,
				},
			},
		}
		r.statusText.Refresh()
		return
	}

	r.port = port
	r.isConnected = true
	r.connectBtn.SetText("Отключиться")
	r.statusText.Segments = []widget.RichTextSegment{
		&widget.TextSegment{
			Text: "Подключено",
			Style: widget.RichTextStyle{
				Color: color.RGBA{R: 0, G: 180, B: 0, A: 255},
				Inline: true,
			},
		},
	}
	r.statusText.Refresh()

	// Запуск чтения данных в горутине
	go r.readData()
}

func (r *RFIDLogger) disconnect() {
	if r.port != nil {
		r.port.Close()
		r.port = nil
	}
	r.isConnected = false
	r.connectBtn.SetText("Подключиться")
	r.statusText.Segments = []widget.RichTextSegment{
		&widget.TextSegment{
			Text: "Отключено",
			Style: widget.RichTextStyle{
				Color: theme.ColorRed,
				Inline: true,
			},
		},
	}
	r.statusText.Refresh()
}

func (r *RFIDLogger) readData() {
	scanner := bufio.NewScanner(r.port)
	for scanner.Scan() {
		line := scanner.Text()
		timestamp := time.Now().Format("15:04:05")
		
		// Добавляем новую строку в лог
		r.logText.Segments = append(r.logText.Segments, &widget.TextSegment{
			Text: fmt.Sprintf("[%s] %s\n", timestamp, line),
			Style: widget.RichTextStyleInline,
		})
		r.logText.Refresh()
		
		// Прокрутка вниз
		r.logScroll.ScrollToBottom()
		
		// Ограничиваем количество строк в логе
		if len(r.logText.Segments) > 200 {
			r.logText.Segments = r.logText.Segments[len(r.logText.Segments)-200:]
		}
	}

	if err := scanner.Err(); err != nil && r.isConnected {
		r.statusText.Segments = []widget.RichTextSegment{
			&widget.TextSegment{
				Text: fmt.Sprintf("Ошибка чтения: %v", err),
				Style: widget.RichTextStyle{
					Color: theme.ColorRed,
					Inline: true,
				},
			},
		}
		r.statusText.Refresh()
	}
}

func (r *RFIDLogger) formatRFIDLine(line string) string {
	// Простая эвристика для выделения UID карт
	// Обычно RFID-RC522 выводит UID в формате шестнадцатеричных чисел
	if strings.Contains(strings.ToLower(line), "uid") || 
	   len(strings.Fields(line)) > 0 && isHexLine(line) {
		return fmt.Sprintf("** RFID: %s **", line)
	}
	return line
}

func isHexLine(line string) bool {
	fields := strings.Fields(line)
	if len(fields) == 0 {
		return false
	}
	// Проверяем, состоит ли строка из шестнадцатеричных значений
	for _, field := range fields {
		for _, ch := range field {
			if !((ch >= '0' && ch <= '9') || (ch >= 'A' && ch <= 'F') || (ch >= 'a' && ch <= 'f')) {
				return false
			}
		}
	}
	return true
}

func createMainWindow() fyne.Window {
	myApp := app.New()
	myWindow := myApp.NewWindow("Arduino RFID Logger")
	myWindow.Resize(fyne.NewSize(600, 400))

	logger := NewRFIDLogger()

	// Создание виджетов
	logger.logText = widget.NewRichText()
	logger.logScroll = container.NewVScroll(logger.logText)

	logger.statusText = widget.NewRichText(
		&widget.TextSegment{
			Text: "Статус: Отключено",
			Style: widget.RichTextStyle{
				Bold: true,
				Inline: true,
			},
		},
	)

	logger.portEntry = widget.NewEntry()
	logger.portEntry.SetPlaceHolder("/dev/ttyUSB0 или COM3")
	logger.portEntry.SetText("/dev/ttyUSB0")

	logger.baudEntry = widget.NewEntry()
	logger.baudEntry.SetPlaceHolder("9600")
	logger.baudEntry.SetText("9600")

	logger.connectBtn = widget.NewButton("Подключиться", logger.connect)

	// Панель настроек подключения
	settingsForm := container.NewVBox(
		widget.NewForm(
			widget.NewFormItem("Порт:", logger.portEntry),
			widget.NewFormItem("Скорость (бод):", logger.baudEntry),
		),
		logger.connectBtn,
		logger.statusText,
	)

	// Заголовок лога
	logHeader := widget.NewLabel("Лог сообщений от Arduino:")
	logHeader.TextStyle = fyne.TextStyle{Bold: true}

	// Основная компоновка
	content := container.NewBorder(
		container.NewVBox(
			widget.NewSeparator(),
			settingsForm,
			widget.NewSeparator(),
			logHeader,
		),
		nil,
		nil,
		nil,
		logger.logScroll,
	)

	// Добавляем меню
	mainMenu := fyne.NewMainMenu(
		fyne.NewMenu("Главное",
			fyne.NewMenuItem("О программе", func() {
				d := dialog.NewInformation("О программе", 
					"Arduino RFID Logger\nВерсия: 1.0.0\nПриложение для чтения логов RFID-меток с Arduino Nano + RC522",
					myWindow)
				d.Show()
			}),
			fyne.NewMenuItem("Выход", func() {
				if logger.isConnected {
					logger.disconnect()
				}
				myApp.Quit()
			}),
		),
	)

	myWindow.SetMainMenu(mainMenu)
	myWindow.SetContent(content)

	return myWindow
}

func main() {
	myWindow := createMainWindow()
	myWindow.ShowAndRun()
}
