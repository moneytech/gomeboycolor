package cartridge

import (
	"bufio"
	"constants"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"strings"
	"types"
	"utils"
)

//Represents MBC1
type MBC1 struct {
	Name            string
	romBank0        []byte
	romBanks        [][]byte
	ramBanks        [][]byte
	selectedROMBank int
	selectedRAMBank int
	hasRAM          bool
	ramEnabled      bool
	hasBattery      bool
	MaxMemMode      int
	ROMSize         int
	RAMSize         int
}

func NewMBC1(rom []byte, romSize int, ramSize int, hasBattery bool) *MBC1 {
	var m *MBC1 = new(MBC1)

	m.Name = "CARTRIDGE-MBC1"
	m.MaxMemMode = constants.SIXTEENMB_ROM_8KBRAM
	m.hasBattery = hasBattery
	m.ROMSize = romSize
	m.RAMSize = ramSize

	if ramSize > 0 {
		m.hasRAM = true
		m.ramEnabled = true
		m.selectedRAMBank = 0
		m.ramBanks = populateRAMBanks(4)
	}

	m.selectedROMBank = 0
	m.romBank0 = rom[0x0000:0x4000]
	m.romBanks = populateROMBanks(rom, m.ROMSize/0x4000)

	return m
}

func (m *MBC1) String() string {
	var batteryStr string
	if m.hasBattery {
		batteryStr += "Yes"
	} else {
		batteryStr += "No"
	}

	return fmt.Sprintln("\nMemory Bank Controller") +
		fmt.Sprintln(strings.Repeat("-", 50)) +
		fmt.Sprintln(utils.PadRight("ROM Banks:", 18, " "), len(m.romBanks), fmt.Sprintf("(%d bytes)", m.ROMSize)) +
		fmt.Sprintln(utils.PadRight("RAM Banks:", 18, " "), m.RAMSize/0x2000, fmt.Sprintf("(%d bytes)", m.RAMSize)) +
		fmt.Sprintln(utils.PadRight("Battery:", 18, " "), batteryStr)
}

func (m *MBC1) Write(addr types.Word, value byte) {
	switch {
	case addr >= 0x0000 && addr <= 0x1FFF:
		//when in 4/32 mode...
		if m.MaxMemMode == constants.FOURMB_ROM_32KBRAM && m.hasRAM {
			if r := value & 0x0F; r == 0x0A {
				log.Println(m.Name + ": Enabling RAM")
				m.ramEnabled = true
			} else {
				log.Println(m.Name + ": Disabling RAM")
				m.ramEnabled = false
			}
		}
	case addr >= 0x2000 && addr <= 0x3FFF:
		m.switchROMBank(int(value & 0x1F))
	case addr >= 0x4000 && addr <= 0x5FFF:
		m.switchRAMBank(int(value & 0x03))
	case addr >= 0x6000 && addr <= 0x7FFF:
		if mode := value & 0x01; mode == 0x00 {
			m.MaxMemMode = constants.SIXTEENMB_ROM_8KBRAM
			log.Println(m.Name + ": Switched MBC1 mode to 16/8")
		} else {
			m.MaxMemMode = constants.FOURMB_ROM_32KBRAM
			log.Println(m.Name + ": Switched MBC1 mode to 4/32")
		}
	case addr >= 0xA000 && addr <= 0xBFFF:
		if m.hasRAM && m.ramEnabled {
			switch m.MaxMemMode {
			case constants.FOURMB_ROM_32KBRAM:
				m.ramBanks[m.selectedRAMBank][addr-0xA000] = value
			case constants.SIXTEENMB_ROM_8KBRAM:
				m.ramBanks[0][addr-0xA000] = value
			}
		}
	}
}

func (m *MBC1) Read(addr types.Word) byte {
	//ROM Bank 0
	if addr < 0x4000 {
		return m.romBank0[addr]
	}

	//Switchable ROM BANK
	if addr >= 0x4000 && addr < 0x8000 {
		return m.romBanks[m.selectedROMBank][addr-0x4000]
	}

	//Upper bounds of memory map.
	if addr >= 0xA000 && addr <= 0xC000 {
		if m.hasRAM && m.ramEnabled {
			switch m.MaxMemMode {
			case constants.FOURMB_ROM_32KBRAM:
				return m.ramBanks[m.selectedRAMBank][addr-0xA000]
			case constants.SIXTEENMB_ROM_8KBRAM:
				return m.ramBanks[0][addr-0xA000]
			}
		}
	}

	return 0x00
}

func (m *MBC1) switchROMBank(bank int) {
	m.selectedROMBank = bank
}

func (m *MBC1) switchRAMBank(bank int) {
	m.selectedRAMBank = bank
}

func (m *MBC1) SaveRam(filename string) error {
	if m.hasRAM && m.hasBattery {
		file, err := os.Create(filename)
		if err != nil {
			return err
		}
		defer file.Close()

		writer := bufio.NewWriter(file)
		log.Println(m.Name+":", "Saving RAM to", filename)
		for i := 0; i < len(m.ramBanks); i++ {
			log.Println(m.Name+":", "--> Saving RAM bank", i)
			writer.Write(m.ramBanks[i])
		}
		writer.Flush()
	}

	return nil
}

func (m *MBC1) LoadRam(filename string) error {
	if m.hasRAM && m.hasBattery {
		fileBytes, err := ioutil.ReadFile(filename)
		if err != nil {
			if os.IsNotExist(err) {
				log.Println(m.Name+":", "Could not find a file named", filename, "on disk. RAM will be empty.")
				return nil
			}
			return err
		}
		log.Println(m.Name+":", "Loading RAM from", filename)

		if len(fileBytes) != 0x8000 {
			return errors.New("RAM file is not 32768 bytes!")
		}

		var chunk types.Word = 0x0000
		for i := 0; i < 4; i++ {
			log.Println(m.Name+":", "--> Populating RAM bank", i)
			m.ramBanks[i] = fileBytes[chunk : chunk+0x2000]
			chunk += 0x2000
		}
	}
	return nil
}