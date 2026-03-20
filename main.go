package main

import (
	"bufio"
	"fmt"
	"image/color"
	"log"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	"github.com/jacobsa/go-serial/serial"
)

// RFIDLogger хранит состояние приложения
type RFIDLogger struct {
	port       serial.Port
	isConnected bool
	logText    *widget.Label
	statusLabel *widget.Label
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
		r.statusLabel.SetText(fmt.Sprintf("Ошибка подключения: %v", err))
		r.statusLabel.Color = theme.ErrorColor()
		r.statusLabel.Refresh()
		return
	}

	r.port = port
	r.isConnected = true
	r.connectBtn.SetText("Отключиться")
	r.statusLabel.SetText("Подключено")
	r.statusLabel.Color = color.RGBA{R: 0, G: 180, B: 0, A: 255}
	r.statusLabel.Refresh()

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
	r.statusLabel.SetText("Отключено")
	r.statusLabel.Color = theme.ErrorColor()
	r.statusLabel.Refresh()
}

func (r *RFIDLogger) readData() {
	scanner := bufio.NewScanner(r.port)
	for scanner.Scan() {
		line := scanner.Text()
		timestamp := time.Now().Format("15:04:05")
		
		// Форматирование вывода с подсветкой UID
		formattedLine := r.formatRFIDLine(line)
		
		r.logText.SetText(fmt.Sprintf("[%s] %s\n%s", timestamp, line, r.logText.Text))
		
		// Ограничиваем количество строк в логе
		lines := strings.Split(r.logText.Text, "\n")
		if len(lines) > 100 {
			r.logText.SetText(strings.Join(lines[:100], "\n"))
		}
	}

	if err := scanner.Err(); err != nil && r.isConnected {
		r.statusLabel.SetText(fmt.Sprintf("Ошибка чтения: %v", err))
		r.statusLabel.Color = theme.ErrorColor()
		r.statusLabel.Refresh()
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
	logger.logText = widget.NewLabel("Ожидание подключения к Arduino...")
	logger.logText.Wrapping = fyne.TextWrapWord
	logger.logText.Scroll = container.ScrollVertical

	logger.statusLabel = widget.NewLabel("Статус: Отключено")
	logger.statusLabel.TextStyle = fyne.TextStyle{Bold: true}

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
		logger.statusLabel,
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
		logger.logText,
	)

	// Добавляем меню
	mainMenu := fyne.NewMenu("Главное",
		fyne.NewMenuItem("О программе", func() {
			dialog := widget.NewModalPopup(
				container.NewVBox(
					widget.NewLabel("Arduino RFID Logger"),
					widget.NewLabel("Версия: 1.0.0"),
					widget.NewLabel("Приложение для чтения логов RFID-меток с Arduino Nano + RC522"),
					widget.NewButton("Закрыть", func() {
						dialog.Hide()
					}),
				),
				myWindow,
			)
			dialog.Show()
		}),
		fyne.NewMenuItem("Выход", func() {
			if logger.isConnected {
				logger.disconnect()
			}
			myApp.Quit()
		}),
	)

	myWindow.SetMainMenu(mainMenu)
	myWindow.SetContent(content)

	return myWindow
}

func main() {
	myWindow := createMainWindow()
	myWindow.ShowAndRun()
}
