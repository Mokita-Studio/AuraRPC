// Command buildres generates the Windows resources for the binary: a
// multi-resolution .ico for external use (installer, docs) and a COFF
// .syso that the Go toolchain links into the binary so the executable
// carries its icon and a DPI-aware manifest.
//
// Run manually after updating any PNG under icons\:
//
//	go run ./tools/buildres
package main

import (
	"fmt"
	"image"
	"image/png"
	"log"
	"os"
	"path/filepath"

	"github.com/tc-hib/winres"
)

func main() {
	root, err := os.Getwd()
	if err != nil {
		log.Fatalf("getwd: %v", err)
	}
	// Allow running from the repo root or from tools\buildres; walk up
	// until we find the icons\ directory.
	for i := 0; i < 3; i++ {
		if _, err := os.Stat(filepath.Join(root, "icons")); err == nil {
			break
		}
		root = filepath.Dir(root)
	}
	pngDir := filepath.Join(root, "icons")
	icoPath := filepath.Join(pngDir, "AuraRPC.ico")
	sysoPath := filepath.Join(root, "cmd", "app", "rsrc_windows_amd64.syso")

	imgs, err := loadPNGs(pngDir, []string{"W16x16.png", "W32x32.png", "W48x48.png", "W256x256.png"})
	if err != nil {
		log.Fatalf("load pngs: %v", err)
	}
	icon, err := winres.NewIconFromImages(imgs)
	if err != nil {
		log.Fatalf("build icon: %v", err)
	}

	// 1) Standalone ICO for the installer and inspection.
	if err := saveICO(icon, icoPath); err != nil {
		log.Fatalf("save ico: %v", err)
	}
	log.Printf("wrote %s", icoPath)

	// 2) Resource set with the icon under numeric ID 1 (Explorer picks
	//    the lowest numeric ID) plus a DPI-aware manifest.
	rs := &winres.ResourceSet{}
	if err := rs.SetIcon(winres.ID(1), icon); err != nil {
		log.Fatalf("set icon: %v", err)
	}
	rs.SetManifest(winres.AppManifest{
		Identity:            winres.AssemblyIdentity{Name: "AuraRPC", Version: [4]uint16{1, 0, 0, 0}},
		Description:         "AuraRPC — Discord Rich Presence",
		DPIAwareness:        winres.DPIPerMonitorV2,
		UseCommonControlsV6: true,
	})

	f, err := os.Create(sysoPath)
	if err != nil {
		log.Fatalf("create syso: %v", err)
	}
	defer f.Close()
	if err := rs.WriteObject(f, winres.ArchAMD64); err != nil {
		log.Fatalf("write syso: %v", err)
	}
	log.Printf("wrote %s", sysoPath)
}

func loadPNGs(dir string, names []string) ([]image.Image, error) {
	imgs := make([]image.Image, 0, len(names))
	for _, n := range names {
		p := filepath.Join(dir, n)
		f, err := os.Open(p)
		if err != nil {
			return nil, fmt.Errorf("open %s: %w", p, err)
		}
		img, err := png.Decode(f)
		f.Close()
		if err != nil {
			return nil, fmt.Errorf("decode %s: %w", p, err)
		}
		imgs = append(imgs, img)
	}
	return imgs, nil
}

func saveICO(icon *winres.Icon, path string) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	return icon.SaveICO(f)
}
