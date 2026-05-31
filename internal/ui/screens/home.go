// Package screens implements the single Home view of AuraRPC: topbar,
// presets sidebar, sectioned editor and a bottom action bar.
package screens

import (
	"image"
	"image/color"
	"log"
	"math"
	"strconv"
	"strings"
	"time"

	"gioui.org/f32"
	"gioui.org/font"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"

	"aurarpc/internal/config"
	"aurarpc/internal/core"
	"aurarpc/internal/i18n"
	"aurarpc/internal/platform"
	"aurarpc/internal/preset"
	"aurarpc/internal/ui/theme"
	"aurarpc/internal/update"
)

// Home is AuraRPC's single screen.
type Home struct {
	mgr     *preset.Manager
	ctrl    *core.Controller
	cfg     *config.Config
	cfgDir  string
	th      *material.Theme
	l       *i18n.Localizer
	updates *update.State

	// OnThemeChange is set by Manager so the material.Theme palette is
	// refreshed when the user toggles light/dark.
	OnThemeChange func()

	// Form widgets — one per preset field.
	presetName widget.Editor
	clientID   widget.Editor
	typeEnum   widget.Enum
	dispEnum   widget.Enum
	appName    widget.Editor
	details    widget.Editor
	detailsURL widget.Editor
	state      widget.Editor
	stateURL   widget.Editor
	partyCur   widget.Editor
	partyMax   widget.Editor
	timeEnum   widget.Enum
	timeStart  widget.Editor
	timeEndOn  widget.Bool
	timeEnd    widget.Editor
	largeKey   widget.Editor
	largeText  widget.Editor
	largeURL   widget.Editor
	smallKey   widget.Editor
	smallText  widget.Editor
	smallURL   widget.Editor
	btn1Text   widget.Editor
	btn1URL    widget.Editor
	btn2Text   widget.Editor
	btn2URL    widget.Editor

	// Topbar.
	sidebarBtn widget.Clickable

	// Sidebar.
	newBtn        widget.Clickable
	presetList    widget.List
	presetClicks  []widget.Clickable
	presetDeletes []widget.Clickable

	// Editor scroll state.
	editorList widget.List

	// Dropdowns (Type / Display / Language).
	typeDD dropdown
	dispDD dropdown
	langDD dropdown

	// Action bar.
	connectBtn  widget.Clickable
	saveBtn     widget.Clickable
	themeBtn    widget.Clickable
	updateBtn   widget.Clickable
	settingsBtn widget.Clickable

	// Settings popover (app-level preferences).
	settingsOpen bool
	autoStart    widget.Bool
	startMin     widget.Bool
	checkUpd     widget.Bool

	// Update-available banner.
	updateLink      widget.Clickable
	updateDismissed widget.Clickable
	updateHidden    bool

	// Runtime state.
	statusText  string
	editingID   string
	saveFlash   time.Time
	updateFlash time.Time

	// Error banner: populated when Save/Apply/Update fail validation;
	// cleared when the user dismisses it or saves successfully.
	errorBanner    string
	errorDismissed widget.Clickable

	// editorRowsList caches the editor section method values so the
	// slice is not reallocated on every frame.
	editorRowsList []func(layout.Context) layout.Dimensions
}

// NewHome builds the Home screen.
func NewHome(mgr *preset.Manager, ctrl *core.Controller, cfg *config.Config, cfgDir string, th *material.Theme, l *i18n.Localizer, updates *update.State) *Home {
	h := &Home{mgr: mgr, ctrl: ctrl, cfg: cfg, cfgDir: cfgDir, th: th, l: l, updates: updates}

	// Seed the settings toggles. Auto-start's live state comes from the OS
	// and is reconciled into config; the others are plain config
	// preferences. config.Apply/Read take the config lock so this never
	// races with the background writer of LastPresetID.
	h.startMin.Value = cfg.StartMinimized
	h.checkUpd.Value = cfg.CheckUpdates
	if on, err := platform.IsAutoStartEnabled(); err == nil {
		h.autoStart.Value = on
		_ = config.Apply(cfgDir, cfg, func(c *config.Config) { c.StartWithSystem = on })
	} else {
		log.Printf("home: read autostart state: %v", err)
		config.Read(cfg, func(c *config.Config) { h.autoStart.Value = c.StartWithSystem })
	}

	var lastPresetID string
	config.Read(cfg, func(c *config.Config) { lastPresetID = c.LastPresetID })
	editors := []*widget.Editor{
		&h.presetName, &h.clientID, &h.appName, &h.details, &h.detailsURL,
		&h.state, &h.stateURL, &h.partyCur, &h.partyMax,
		&h.timeStart, &h.timeEnd,
		&h.largeKey, &h.largeText, &h.largeURL,
		&h.smallKey, &h.smallText, &h.smallURL,
		&h.btn1Text, &h.btn1URL, &h.btn2Text, &h.btn2URL,
	}
	for _, e := range editors {
		e.SingleLine = true
	}
	h.presetList.Axis = layout.Vertical
	h.editorList.Axis = layout.Vertical

	// Preallocate the editor row slice: method values capture h and
	// never change between frames, so building the list once avoids
	// ~60 allocs/second on the Gio hot path.
	h.editorRowsList = []func(layout.Context) layout.Dimensions{
		h.editorHead,
		h.sectionIdentity,
		h.sectionActivity,
		h.sectionParty,
		h.sectionTime,
		h.sectionLargeImage,
		h.sectionSmallImage,
		h.sectionButtons,
		func(gtx layout.Context) layout.Dimensions {
			return layout.Spacer{Height: unit.Dp(12)}.Layout(gtx)
		},
	}

	// Defaults — overridden when loading a preset.
	h.typeEnum.Value = string(preset.TypePlaying)
	h.dispEnum.Value = string(preset.DisplayName)
	h.timeEnum.Value = string(preset.TimeNone)

	if list := mgr.List(); len(list) > 0 {
		var target *preset.Preset
		for _, p := range list {
			if p.ID == lastPresetID {
				target = p
				break
			}
		}
		if target == nil {
			target = list[0]
		}
		h.loadInto(target)
	}
	return h
}

// ───────── Main layout ─────────

func (h *Home) Layout(gtx layout.Context) layout.Dimensions {
	h.handleClicks(gtx)
	h.statusText = h.ctrl.Status()

	paint.FillShape(gtx.Ops, theme.Background, clip.Rect{Max: gtx.Constraints.Max}.Op())

	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(h.topbar),
		layout.Rigid(h.updateBannerWidget),
		layout.Rigid(h.errorBannerWidget),
		layout.Flexed(1, h.body),
		layout.Rigid(h.actionbar),
	)
}

// updateAvailable returns the pending release, if the background check
// found a newer one and the banner has not been dismissed.
func (h *Home) updateAvailable() (update.Release, bool) {
	if h.updates == nil {
		return update.Release{}, false
	}
	return h.updates.Available()
}

// ───────── Topbar ─────────

const topbarHeightDp = 40

func (h *Home) topbar(gtx layout.Context) layout.Dimensions {
	height := gtx.Dp(unit.Dp(topbarHeightDp))
	paint.FillShape(gtx.Ops, theme.Chrome, clip.Rect{Max: image.Pt(gtx.Constraints.Max.X, height)}.Op())
	drawBottomBorder(gtx, image.Pt(gtx.Constraints.Max.X, height), theme.Divider)

	gtxBar := gtx
	gtxBar.Constraints.Min.Y = height
	gtxBar.Constraints.Max.Y = height

	hamburger := h.iconBtn(&h.sidebarBtn, h.drawHamburger, h.cfg.SidebarOpen)

	return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtxBar,
		layout.Rigid(hamburger),
		layout.Rigid(layout.Spacer{Width: unit.Dp(4)}.Layout),
		layout.Rigid(h.brand),
	)
}

// brand draws the topbar mark (name only, no subtitle), vertically
// centered against the full bar height.
func (h *Home) brand(gtx layout.Context) layout.Dimensions {
	mark := func(gtx layout.Context) layout.Dimensions {
		s := gtx.Dp(unit.Dp(6))
		// Vertically center within the topbar height.
		y := (gtx.Constraints.Max.Y - s) / 2
		defer op.Offset(image.Pt(0, y)).Push(gtx.Ops).Pop()
		paint.FillShape(gtx.Ops, theme.Accent, clip.Rect{Max: image.Pt(s, s)}.Op())
		return layout.Dimensions{Size: image.Pt(s, gtx.Constraints.Max.Y)}
	}
	name := material.Body2(h.th, h.l.T("app.title"))
	name.TextSize = unit.Sp(13)
	name.Font.Weight = font.Medium
	name.Color = theme.TextPrimary

	return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
		layout.Rigid(mark),
		layout.Rigid(layout.Spacer{Width: unit.Dp(8)}.Layout),
		layout.Rigid(centerY(name.Layout)),
	)
}

// centerY wraps a widget to vertically center it within fixed-height
// rows (topbar, actionbar). Only Min.Y is zeroed so Flexed children
// keep their assigned cell width and Spacing has room to distribute.
func centerY(w layout.Widget) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		macro := op.Record(gtx.Ops)
		inner := gtx
		inner.Constraints.Min.Y = 0
		dims := w(inner)
		call := macro.Stop()
		y := (gtx.Constraints.Max.Y - dims.Size.Y) / 2
		if y < 0 {
			y = 0
		}
		defer op.Offset(image.Pt(0, y)).Push(gtx.Ops).Pop()
		call.Add(gtx.Ops)
		return layout.Dimensions{Size: image.Pt(dims.Size.X, gtx.Constraints.Max.Y)}
	}
}

// ───────── Body (sidebar + editor) ─────────

func (h *Home) body(gtx layout.Context) layout.Dimensions {
	if !h.cfg.SidebarOpen {
		return h.editor(gtx)
	}
	sidebarW := gtx.Dp(unit.Dp(200))
	return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			gtx.Constraints.Max.X = sidebarW
			gtx.Constraints.Min.X = sidebarW
			return h.sidebar(gtx)
		}),
		layout.Rigid(verticalDivider),
		layout.Flexed(1, h.editor),
	)
}

// ───────── Sidebar ─────────

func (h *Home) sidebar(gtx layout.Context) layout.Dimensions {
	return layout.Inset{Top: unit.Dp(16), Bottom: unit.Dp(12)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(h.sidebarHead),
			layout.Rigid(layout.Spacer{Height: unit.Dp(8)}.Layout),
			layout.Rigid(h.sidebarNew),
			layout.Flexed(1, h.sidebarPresets),
		)
	})
}

func (h *Home) sidebarHead(gtx layout.Context) layout.Dimensions {
	title := material.Caption(h.th, strings.ToUpper(h.l.T("sidebar.title")))
	title.Color = theme.TextSecondary
	title.Font.Weight = font.Medium
	title.TextSize = unit.Sp(11)

	sub := material.Caption(h.th, h.l.T("sidebar.subtitle"))
	sub.Color = theme.TextMuted
	sub.TextSize = unit.Sp(11)

	return layout.Inset{Left: unit.Dp(16), Right: unit.Dp(16)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(title.Layout),
			layout.Rigid(sub.Layout),
		)
	})
}

func (h *Home) sidebarNew(gtx layout.Context) layout.Dimensions {
	return h.newBtn.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		hovered := h.newBtn.Hovered()
		if hovered {
			paint.FillShape(gtx.Ops, hoverColor(), clip.Rect{Max: gtx.Constraints.Max}.Op())
		}
		return layout.Inset{Top: unit.Dp(8), Bottom: unit.Dp(8), Left: unit.Dp(16), Right: unit.Dp(16)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			lbl := material.Body2(h.th, "+  "+h.l.T("sidebar.new"))
			if hovered {
				lbl.Color = theme.TextPrimary
			} else {
				lbl.Color = theme.TextSecondary
			}
			lbl.TextSize = unit.Sp(12)
			return lbl.Layout(gtx)
		})
	})
}

func (h *Home) sidebarPresets(gtx layout.Context) layout.Dimensions {
	list := h.mgr.List()
	for len(h.presetClicks) < len(list) {
		h.presetClicks = append(h.presetClicks, widget.Clickable{})
		h.presetDeletes = append(h.presetDeletes, widget.Clickable{})
	}

	if len(list) == 0 {
		lbl := material.Caption(h.th, h.l.T("sidebar.empty"))
		lbl.Color = theme.TextMuted
		return layout.Inset{Left: unit.Dp(16), Right: unit.Dp(16), Top: unit.Dp(12)}.Layout(gtx, lbl.Layout)
	}

	return material.List(h.th, &h.presetList).Layout(gtx, len(list), func(gtx layout.Context, i int) layout.Dimensions {
		return h.presetRow(gtx, list[i], i)
	})
}

func (h *Home) presetRow(gtx layout.Context, p *preset.Preset, i int) layout.Dimensions {
	main := &h.presetClicks[i]
	del := &h.presetDeletes[i]
	active := p.ID == h.editingID
	hovered := main.Hovered()

	return main.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		switch {
		case active:
			paint.FillShape(gtx.Ops, theme.Surface, clip.Rect{Max: gtx.Constraints.Max}.Op())
		case hovered:
			paint.FillShape(gtx.Ops, hoverColor(), clip.Rect{Max: gtx.Constraints.Max}.Op())
		}
		return layout.Inset{Top: unit.Dp(8), Bottom: unit.Dp(8), Left: unit.Dp(16), Right: unit.Dp(8)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					w := gtx.Dp(unit.Dp(2))
					hg := gtx.Dp(unit.Dp(18))
					col := color.NRGBA{}
					if active {
						col = theme.Accent
					}
					paint.FillShape(gtx.Ops, col, clip.Rect{Max: image.Pt(w, hg)}.Op())
					return layout.Dimensions{Size: image.Pt(w, hg)}
				}),
				layout.Rigid(layout.Spacer{Width: unit.Dp(10)}.Layout),
				layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
					nameTxt := p.Name
					if nameTxt == "" {
						nameTxt = h.l.T("sidebar.untitled")
					}
					name := material.Body2(h.th, nameTxt)
					name.MaxLines = 1
					name.TextSize = unit.Sp(13)
					if active {
						name.Color = theme.TextPrimary
						name.Font.Weight = font.Medium
					} else {
						name.Color = theme.TextPrimary
					}

					sub := material.Caption(h.th, p.AppName)
					sub.MaxLines = 1
					sub.Color = theme.TextMuted
					sub.TextSize = unit.Sp(11)

					children := []layout.FlexChild{layout.Rigid(name.Layout)}
					if p.AppName != "" {
						children = append(children, layout.Rigid(sub.Layout))
					}
					return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					if !hovered {
						return layout.Dimensions{}
					}
					return del.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						return layout.UniformInset(unit.Dp(4)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
							c := theme.TextMuted
							if del.Hovered() {
								c = theme.StatusError
							}
							return h.drawTrash(gtx, c)
						})
					})
				}),
			)
		})
	})
}

// ───────── Editor ─────────

func (h *Home) editor(gtx layout.Context) layout.Dimensions {
	// Asymmetric padding compensates for the material.List scrollbar,
	// which is painted inside the right margin. Form content ends up
	// visually equidistant from both window edges.
	return layout.Inset{Left: unit.Dp(32), Right: unit.Dp(16), Top: unit.Dp(24), Bottom: unit.Dp(24)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return material.List(h.th, &h.editorList).Layout(gtx, len(h.editorRowsList), func(gtx layout.Context, i int) layout.Dimensions {
			// Per-row inset so the scrollbar never overlaps a field.
			return layout.Inset{Right: unit.Dp(16)}.Layout(gtx, h.editorRowsList[i])
		})
	})
}

func (h *Home) editorHead(gtx layout.Context) layout.Dimensions {
	return layout.Inset{Bottom: unit.Dp(28)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		ed := material.Editor(h.th, &h.presetName, h.l.T("editor.preset_name.placeholder"))
		ed.TextSize = unit.Sp(18)
		ed.Font.Weight = font.Medium
		ed.Color = theme.TextPrimary
		ed.HintColor = theme.TextMuted

		lbl := material.Caption(h.th, h.l.T("editor.preset_label"))
		lbl.Color = theme.TextMuted
		lbl.TextSize = unit.Sp(11)

		return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Baseline}.Layout(gtx,
			layout.Rigid(ed.Layout),
			layout.Rigid(layout.Spacer{Width: unit.Dp(12)}.Layout),
			layout.Rigid(lbl.Layout),
		)
	})
}

func (h *Home) sectionIdentity(gtx layout.Context) layout.Dimensions {
	return h.section(gtx, h.l.T("section.identity"), h.l.T("section.identity.help"), func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(h.field(h.l.T("field.client_id"), h.l.T("field.client_id.help"), h.monoInput(&h.clientID, h.l.T("field.client_id.placeholder")))),
			layout.Rigid(layout.Spacer{Height: unit.Dp(14)}.Layout),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return h.twoCol(gtx,
					h.field(h.l.T("field.type"), h.l.T("field.type.help"), h.typeDropdown()),
					h.field(h.l.T("field.display"), h.l.T("field.display.help"), h.displayDropdown()),
				)
			}),
		)
	})
}

func (h *Home) sectionActivity(gtx layout.Context) layout.Dimensions {
	return h.section(gtx, h.l.T("section.activity"), h.l.T("section.activity.help"), func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(h.field(h.l.T("field.name"), h.l.T("field.name.help"), h.textInput(&h.appName, h.l.T("field.name.placeholder")))),
			layout.Rigid(layout.Spacer{Height: unit.Dp(14)}.Layout),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return h.twoColWeighted(gtx, 7, 5,
					h.field(h.l.T("field.details"), h.l.T("field.details.help"), h.textInput(&h.details, h.l.T("field.details.placeholder"))),
					h.field(h.l.T("field.url"), h.l.T("field.url.help"), h.textInput(&h.detailsURL, h.l.T("field.url.placeholder"))),
				)
			}),
			layout.Rigid(layout.Spacer{Height: unit.Dp(14)}.Layout),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return h.twoColWeighted(gtx, 7, 5,
					h.field(h.l.T("field.state"), h.l.T("field.state.help"), h.textInput(&h.state, h.l.T("field.state.placeholder"))),
					h.field(h.l.T("field.url"), h.l.T("field.url.help"), h.textInput(&h.stateURL, h.l.T("field.url.placeholder"))),
				)
			}),
		)
	})
}

func (h *Home) sectionParty(gtx layout.Context) layout.Dimensions {
	return h.section(gtx, h.l.T("section.party"), h.l.T("section.party.help"), func(gtx layout.Context) layout.Dimensions {
		return h.twoCol(gtx,
			h.field(h.l.T("field.party_current"), h.l.T("field.party_current.help"), h.textInput(&h.partyCur, "0")),
			h.field(h.l.T("field.party_max"), h.l.T("field.party_max.help"), h.textInput(&h.partyMax, "0")),
		)
	})
}

func (h *Home) sectionTime(gtx layout.Context) layout.Dimensions {
	return h.section(gtx, h.l.T("section.time"), h.l.T("section.time.help"), func(gtx layout.Context) layout.Dimensions {
		modes := []struct{ id, label, help string }{
			{string(preset.TimeNone), h.l.T("time.none"), h.l.T("time.none.help")},
			{string(preset.TimeSinceConnect), h.l.T("time.since_connect"), h.l.T("time.since_connect.help")},
			{string(preset.TimeSincePresence), h.l.T("time.since_presence"), h.l.T("time.since_presence.help")},
			{string(preset.TimeSinceStart), h.l.T("time.since_start"), h.l.T("time.since_start.help")},
			{string(preset.TimeLocal), h.l.T("time.local"), h.l.T("time.local.help")},
			{string(preset.TimeCustom), h.l.T("time.custom"), h.l.T("time.custom.help")},
		}
		children := make([]layout.FlexChild, 0, len(modes)+3)
		for _, m := range modes {
			m := m
			children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return h.radioRow(gtx, &h.timeEnum, m.id, m.label, m.help)
			}))
		}
		if h.timeEnum.Value == string(preset.TimeCustom) {
			children = append(children,
				layout.Rigid(layout.Spacer{Height: unit.Dp(14)}.Layout),
				layout.Rigid(h.dividerHorizontal),
				layout.Rigid(layout.Spacer{Height: unit.Dp(12)}.Layout),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return h.twoCol(gtx,
						h.field(h.l.T("time.start"), "", h.textInput(&h.timeStart, h.l.T("time.placeholder"))),
						func(gtx layout.Context) layout.Dimensions {
							return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
								layout.Rigid(func(gtx layout.Context) layout.Dimensions {
									lbl := material.Caption(h.th, h.l.T("time.end"))
									lbl.Color = theme.TextSecondary
									lbl.Font.Weight = font.Medium
									lbl.TextSize = unit.Sp(11)
									return lbl.Layout(gtx)
								}),
								layout.Rigid(layout.Spacer{Height: unit.Dp(4)}.Layout),
								layout.Rigid(func(gtx layout.Context) layout.Dimensions {
									return h.checkbox(gtx, &h.timeEndOn, h.l.T("time.end_enable"))
								}),
								layout.Rigid(func(gtx layout.Context) layout.Dimensions {
									if !h.timeEndOn.Value {
										return layout.Dimensions{}
									}
									return layout.Inset{Top: unit.Dp(6)}.Layout(gtx, h.textInput(&h.timeEnd, h.l.T("time.placeholder")))
								}),
							)
						},
					)
				}),
			)
		}
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
	})
}

func (h *Home) sectionLargeImage(gtx layout.Context) layout.Dimensions {
	return h.imageSection(gtx, h.l.T("section.large"), h.l.T("section.large.help"),
		&h.largeKey, &h.largeText, &h.largeURL,
		h.l.T("field.image_text.large_placeholder"))
}

func (h *Home) sectionSmallImage(gtx layout.Context) layout.Dimensions {
	return h.imageSection(gtx, h.l.T("section.small"), h.l.T("section.small.help"),
		&h.smallKey, &h.smallText, &h.smallURL,
		h.l.T("field.image_text.small_placeholder"))
}

func (h *Home) imageSection(gtx layout.Context, title, help string, key, txt, url *widget.Editor, txtPh string) layout.Dimensions {
	return h.section(gtx, title, help, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(h.field(h.l.T("field.image_key"), h.l.T("field.image_key.help"), h.textInput(key, h.l.T("field.image_key.placeholder")))),
			layout.Rigid(layout.Spacer{Height: unit.Dp(14)}.Layout),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return h.twoColWeighted(gtx, 7, 5,
					h.field(h.l.T("field.image_text"), h.l.T("field.image_text.help"), h.textInput(txt, txtPh)),
					h.field(h.l.T("field.image_url"), h.l.T("field.image_url.help"), h.textInput(url, h.l.T("field.url.placeholder"))),
				)
			}),
		)
	})
}

func (h *Home) sectionButtons(gtx layout.Context) layout.Dimensions {
	return h.section(gtx, h.l.T("section.buttons"), h.l.T("section.buttons.help"), func(gtx layout.Context) layout.Dimensions {
		return h.twoCol(gtx,
			h.buttonBlock(h.l.T("field.button1"), &h.btn1Text, &h.btn1URL),
			h.buttonBlock(h.l.T("field.button2"), &h.btn2Text, &h.btn2URL),
		)
	})
}

func (h *Home) buttonBlock(label string, txt, url *widget.Editor) func(layout.Context) layout.Dimensions {
	return func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				lbl := material.Caption(h.th, strings.ToUpper(label))
				lbl.Color = theme.TextSecondary
				lbl.Font.Weight = font.Medium
				lbl.TextSize = unit.Sp(11)
				return lbl.Layout(gtx)
			}),
			layout.Rigid(layout.Spacer{Height: unit.Dp(10)}.Layout),
			layout.Rigid(h.field(h.l.T("field.button_text"), h.l.T("field.button_text.help"), h.textInput(txt, h.l.T("field.button_text.placeholder")))),
			layout.Rigid(layout.Spacer{Height: unit.Dp(12)}.Layout),
			layout.Rigid(h.field(h.l.T("field.button_url"), h.l.T("field.button_url.help"), h.textInput(url, h.l.T("field.url.placeholder")))),
		)
	}
}

// ───────── Section + Field primitives ─────────

func (h *Home) section(gtx layout.Context, title, help string, body layout.Widget) layout.Dimensions {
	return layout.Inset{Bottom: unit.Dp(24)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				lbl := material.Body2(h.th, strings.ToUpper(title))
				lbl.Color = theme.TextPrimary
				lbl.Font.Weight = font.Medium
				lbl.TextSize = unit.Sp(13)
				return lbl.Layout(gtx)
			}),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				if help == "" {
					return layout.Dimensions{}
				}
				lbl := material.Caption(h.th, help)
				lbl.Color = theme.HelpText
				lbl.TextSize = unit.Sp(11)
				return layout.Inset{Top: unit.Dp(2)}.Layout(gtx, lbl.Layout)
			}),
			layout.Rigid(layout.Spacer{Height: unit.Dp(16)}.Layout),
			layout.Rigid(body),
			layout.Rigid(layout.Spacer{Height: unit.Dp(20)}.Layout),
			layout.Rigid(h.dividerHorizontal),
		)
	})
}

func (h *Home) field(label, help string, input layout.Widget) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				lbl := material.Caption(h.th, label)
				lbl.Color = theme.TextSecondary
				lbl.Font.Weight = font.Medium
				lbl.TextSize = unit.Sp(11)
				return lbl.Layout(gtx)
			}),
			layout.Rigid(layout.Spacer{Height: unit.Dp(4)}.Layout),
			layout.Rigid(input),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				if help == "" {
					return layout.Dimensions{}
				}
				lbl := material.Caption(h.th, help)
				lbl.Color = theme.HelpText
				lbl.TextSize = unit.Sp(11)
				return layout.Inset{Top: unit.Dp(4)}.Layout(gtx, lbl.Layout)
			}),
		)
	}
}

func (h *Home) twoCol(gtx layout.Context, left, right layout.Widget) layout.Dimensions {
	return h.twoColWeighted(gtx, 1, 1, left, right)
}

func (h *Home) twoColWeighted(gtx layout.Context, lw, rw float32, left, right layout.Widget) layout.Dimensions {
	return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
		layout.Flexed(lw, left),
		layout.Rigid(layout.Spacer{Width: unit.Dp(16)}.Layout),
		layout.Flexed(rw, right),
	)
}

// ───────── Inputs ─────────

func (h *Home) textInput(ed *widget.Editor, placeholder string) layout.Widget {
	return h.inputWith(ed, placeholder, false)
}

func (h *Home) monoInput(ed *widget.Editor, placeholder string) layout.Widget {
	return h.inputWith(ed, placeholder, true)
}

func (h *Home) inputWith(ed *widget.Editor, placeholder string, mono bool) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		macro := op.Record(gtx.Ops)
		dims := layout.Inset{Top: unit.Dp(7), Bottom: unit.Dp(7), Left: unit.Dp(10), Right: unit.Dp(10)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			style := material.Editor(h.th, ed, placeholder)
			// Match size and visual weight between sans and mono fields.
			// Go Mono glyphs are thinner than Inter's, so Medium keeps
			// the same optical presence as user text.
			style.TextSize = unit.Sp(13)
			if mono {
				style.Font.Typeface = "Go Mono"
				style.Font.Weight = font.Medium
			}
			style.Color = theme.TextPrimary
			style.HintColor = theme.TextMuted
			return style.Layout(gtx)
		})
		content := macro.Stop()
		paint.FillShape(gtx.Ops, theme.Surface, clip.Rect{Max: dims.Size}.Op())
		content.Add(gtx.Ops)
		return dims
	}
}

// ───────── Radio + Checkbox ─────────

func (h *Home) radioRow(gtx layout.Context, en *widget.Enum, value, label, help string) layout.Dimensions {
	return layout.Inset{Top: unit.Dp(4), Bottom: unit.Dp(4)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		btn := en.Update(gtx)
		_ = btn
		// Clickable area
		clickable := en
		_ = clickable

		// Use material.RadioButton for input handling with a custom layout.
		rb := material.RadioButton(h.th, en, value, label)
		rb.Color = theme.TextPrimary
		rb.IconColor = theme.Accent
		rb.TextSize = unit.Sp(13)
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(rb.Layout),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				if help == "" {
					return layout.Dimensions{}
				}
				lbl := material.Caption(h.th, help)
				lbl.Color = theme.HelpText
				lbl.TextSize = unit.Sp(11)
				return layout.Inset{Left: unit.Dp(24), Top: unit.Dp(1)}.Layout(gtx, lbl.Layout)
			}),
		)
	})
}

func (h *Home) checkbox(gtx layout.Context, b *widget.Bool, label string) layout.Dimensions {
	cb := material.CheckBox(h.th, b, label)
	cb.Color = theme.TextPrimary
	cb.IconColor = theme.Accent
	cb.TextSize = unit.Sp(12)
	return cb.Layout(gtx)
}

// ───────── Dropdowns (Type / Display / Language) ─────────

type dropdown struct {
	trigger widget.Clickable
	open    bool
	items   []widget.Clickable
}

func (h *Home) typeDropdown() layout.Widget {
	opts := []ddOption{
		{string(preset.TypePlaying), h.l.T("type.playing"), ""},
		{string(preset.TypeStreaming), h.l.T("type.streaming"), ""},
		{string(preset.TypeListening), h.l.T("type.listening"), ""},
		{string(preset.TypeWatching), h.l.T("type.watching"), ""},
		{string(preset.TypeCompeting), h.l.T("type.competing"), ""},
	}
	return h.renderDropdown(&h.typeDD, h.typeEnum.Value, opts, func(v string) {
		h.typeEnum.Value = v
	})
}

func (h *Home) displayDropdown() layout.Widget {
	opts := []ddOption{
		{string(preset.DisplayName), h.l.T("display.name"), ""},
		{string(preset.DisplayDetails), h.l.T("display.details"), ""},
		{string(preset.DisplayState), h.l.T("display.state"), ""},
	}
	return h.renderDropdown(&h.dispDD, h.dispEnum.Value, opts, func(v string) {
		h.dispEnum.Value = v
	})
}

type ddOption struct {
	value string
	label string
	hint  string
}

// renderDropdown draws an input-styled select that floats its menu via
// op.Defer, so the trigger's height does not change while the menu is open.
func (h *Home) renderDropdown(dd *dropdown, current string, opts []ddOption, onPick func(string)) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		// Grow the clickable pool to match the option count.
		for len(dd.items) < len(opts) {
			dd.items = append(dd.items, widget.Clickable{})
		}
		if dd.trigger.Clicked(gtx) {
			dd.open = !dd.open
		}
		for i := range dd.items {
			if dd.items[i].Clicked(gtx) {
				if i < len(opts) {
					onPick(opts[i].value)
				}
				dd.open = false
			}
		}

		// Visible label.
		var currentLabel string
		for _, o := range opts {
			if o.value == current {
				currentLabel = o.label
				break
			}
		}

		// Trigger. Shares vertical insets with textInput so dropdowns
		// and inputs align in the grid. Right padding is bumped to 12dp
		// to give the chevron some breathing room.
		dims := dd.trigger.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			macro := op.Record(gtx.Ops)
			d := layout.Inset{Top: unit.Dp(7), Bottom: unit.Dp(7), Left: unit.Dp(10), Right: unit.Dp(12)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
					layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
						lbl := material.Body2(h.th, currentLabel)
						lbl.Color = theme.TextPrimary
						lbl.TextSize = unit.Sp(13)
						return lbl.Layout(gtx)
					}),
					layout.Rigid(layout.Spacer{Width: unit.Dp(8)}.Layout),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return h.drawChevron(gtx, theme.TextMuted)
					}),
				)
			})
			content := macro.Stop()
			paint.FillShape(gtx.Ops, theme.Surface, clip.Rect{Max: d.Size}.Op())
			content.Add(gtx.Ops)
			return d
		})

		if dd.open {
			h.deferDropdownMenu(gtx, dd, opts, current, image.Pt(0, dims.Size.Y+2), dims.Size.X)
		}
		return dims
	}
}

// deferDropdownMenu paints the menu with op.Defer so it floats above the
// rest of the frame without changing the trigger's dimensions. offset is
// relative to the trigger origin.
func (h *Home) deferDropdownMenu(gtx layout.Context, dd *dropdown, opts []ddOption, current string, offset image.Point, minWidth int) {
	macro := op.Record(gtx.Ops)
	menuGtx := gtx
	menuGtx.Constraints.Min = image.Point{}
	if minWidth > 0 {
		menuGtx.Constraints.Min.X = minWidth
	}
	// Unbounded vertical constraint: the menu picks its own height.
	menuGtx.Constraints.Max.Y = 1<<31 - 1
	if menuGtx.Constraints.Max.X < menuGtx.Constraints.Min.X {
		menuGtx.Constraints.Max.X = menuGtx.Constraints.Min.X
	}
	tr := op.Offset(offset).Push(menuGtx.Ops)
	h.renderDropdownMenuContent(menuGtx, dd, opts, current)
	tr.Pop()
	call := macro.Stop()
	op.Defer(gtx.Ops, call)
}

func (h *Home) renderDropdownMenuContent(gtx layout.Context, dd *dropdown, opts []ddOption, current string) layout.Dimensions {
	children := make([]layout.FlexChild, 0, len(opts))
	for i, o := range opts {
		i, o := i, o
		children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			clk := &dd.items[i]
			return clk.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				macro := op.Record(gtx.Ops)
				dims := layout.Inset{Top: unit.Dp(8), Bottom: unit.Dp(8), Left: unit.Dp(12), Right: unit.Dp(12)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
						layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
							lbl := material.Body2(h.th, o.label)
							lbl.TextSize = unit.Sp(12)
							if o.value == current {
								lbl.Color = theme.Accent
								lbl.Font.Weight = font.Medium
							} else {
								lbl.Color = theme.TextPrimary
							}
							return lbl.Layout(gtx)
						}),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							if o.hint == "" {
								return layout.Dimensions{}
							}
							lbl := material.Caption(h.th, o.hint)
							lbl.Color = theme.TextMuted
							lbl.TextSize = unit.Sp(10)
							return layout.Inset{Left: unit.Dp(16)}.Layout(gtx, lbl.Layout)
						}),
					)
				})
				content := macro.Stop()
				if clk.Hovered() {
					paint.FillShape(gtx.Ops, hoverColor(), clip.Rect{Max: dims.Size}.Op())
				}
				content.Add(gtx.Ops)
				return dims
			})
		}))
	}
	macro := op.Record(gtx.Ops)
	dims := layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
	content := macro.Stop()
	paint.FillShape(gtx.Ops, theme.Background, clip.Rect{Max: dims.Size}.Op())
	drawBorderAll(gtx, dims.Size, theme.Divider)
	content.Add(gtx.Ops)
	return dims
}

// ───────── Action bar ─────────

const actionbarHeightDp = 40

func (h *Home) actionbar(gtx layout.Context) layout.Dimensions {
	height := gtx.Dp(unit.Dp(actionbarHeightDp))
	paint.FillShape(gtx.Ops, theme.Chrome, clip.Rect{Max: image.Pt(gtx.Constraints.Max.X, height)}.Op())
	drawTopBorder(gtx, image.Pt(gtx.Constraints.Max.X, height), theme.Divider)

	gtxBar := gtx
	gtxBar.Constraints.Min.Y = height
	gtxBar.Constraints.Max.Y = height

	return layout.Inset{Left: unit.Dp(8), Right: unit.Dp(8)}.Layout(gtxBar, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
			layout.Flexed(1, centerY(h.actionLeft)),
			layout.Rigid(centerY(h.actionCenter)),
			layout.Flexed(1, centerY(h.actionRight)),
		)
	})
}

func (h *Home) actionLeft(gtx layout.Context) layout.Dimensions {
	connectLabel := h.l.T("btn.connect")
	if h.statusText == "connected" {
		connectLabel = h.l.T("btn.disconnect")
	} else if h.statusText == "connecting" {
		connectLabel = h.l.T("status.connecting")
	}
	saveLabel := h.l.T("btn.save")
	if !h.saveFlash.IsZero() && time.Since(h.saveFlash) < 1600*time.Millisecond {
		saveLabel = h.l.T("btn.saved")
	}
	return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
		layout.Rigid(h.outlineButton(&h.connectBtn, connectLabel, h.statusText == "connecting")),
		layout.Rigid(layout.Spacer{Width: unit.Dp(6)}.Layout),
		layout.Rigid(h.ghostButton(&h.saveBtn, saveLabel)),
	)
}

func (h *Home) actionCenter(gtx layout.Context) layout.Dimensions {
	statusKey := "status.disconnected"
	switch h.statusText {
	case "connected":
		statusKey = "status.connected"
	case "connecting":
		statusKey = "status.connecting"
	}
	var col color.NRGBA
	switch h.statusText {
	case "connected":
		col = theme.StatusConnected
	case "connecting":
		col = theme.Accent
	default:
		col = theme.TextMuted
	}
	size := gtx.Dp(unit.Dp(8))
	dot := func(gtx layout.Context) layout.Dimensions {
		paint.FillShape(gtx.Ops, col, clip.Ellipse{Max: image.Pt(size, size)}.Op(gtx.Ops))
		return layout.Dimensions{Size: image.Pt(size, size)}
	}
	lbl := material.Body2(h.th, h.l.T(statusKey))
	lbl.Color = theme.TextSecondary
	lbl.TextSize = unit.Sp(12)

	return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
		layout.Rigid(dot),
		layout.Rigid(layout.Spacer{Width: unit.Dp(8)}.Layout),
		layout.Rigid(lbl.Layout),
	)
}

func (h *Home) actionRight(gtx layout.Context) layout.Dimensions {
	updateLabel := h.l.T("btn.update")
	if !h.updateFlash.IsZero() && time.Since(h.updateFlash) < 1800*time.Millisecond {
		updateLabel = h.l.T("btn.updated")
	}
	themeDraw := h.drawMoon
	if theme.Mode() == "dark" {
		themeDraw = h.drawSun
	}
	// Spacing: SpaceStart pushes the group to the right edge of the bar.
	// Visual order left-to-right: theme · settings · language · update.
	return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle, Spacing: layout.SpaceStart}.Layout(gtx,
		layout.Rigid(h.iconBtn(&h.themeBtn, themeDraw, false)),
		layout.Rigid(layout.Spacer{Width: unit.Dp(6)}.Layout),
		layout.Rigid(h.settingsButton()),
		layout.Rigid(layout.Spacer{Width: unit.Dp(6)}.Layout),
		layout.Rigid(h.languageDropdown()),
		layout.Rigid(layout.Spacer{Width: unit.Dp(10)}.Layout),
		layout.Rigid(h.primaryButton(&h.updateBtn, updateLabel, h.statusText != "connected")),
	)
}

// settingsButton renders the gear icon and, when open, floats the app
// preferences panel above it.
func (h *Home) settingsButton() layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		dims := h.iconBtn(&h.settingsBtn, h.drawGear, h.settingsOpen)(gtx)
		if h.settingsOpen {
			h.deferSettingsPanel(gtx)
		}
		return dims
	}
}

// deferSettingsPanel measures the preferences panel, then floats it just
// above the gear via op.Defer so it never grows the action bar.
func (h *Home) deferSettingsPanel(gtx layout.Context) {
	panelW := gtx.Dp(unit.Dp(300))
	panelGtx := gtx
	panelGtx.Constraints.Min = image.Pt(panelW, 0)
	panelGtx.Constraints.Max.X = panelW
	panelGtx.Constraints.Max.Y = 1<<31 - 1

	// Measure pass: ops are recorded and discarded, so no event areas or
	// paint operations leak into the frame — only the height is used.
	measure := op.Record(gtx.Ops)
	dims := h.settingsPanelContent(panelGtx)
	measure.Stop()

	macro := op.Record(gtx.Ops)
	// Right-align the panel to the gear and lift it above the action bar.
	off := image.Pt(-(panelW - gtx.Dp(unit.Dp(32))), -dims.Size.Y-gtx.Dp(unit.Dp(6)))
	tr := op.Offset(off).Push(gtx.Ops)
	h.settingsPanelContent(panelGtx)
	tr.Pop()
	op.Defer(gtx.Ops, macro.Stop())
}

func (h *Home) settingsPanelContent(gtx layout.Context) layout.Dimensions {
	rows := []struct {
		b           *widget.Bool
		label, help string
	}{
		{&h.autoStart, h.l.T("settings.autostart"), h.l.T("settings.autostart.help")},
		{&h.startMin, h.l.T("settings.start_minimized"), h.l.T("settings.start_minimized.help")},
		{&h.checkUpd, h.l.T("settings.check_updates"), h.l.T("settings.check_updates.help")},
	}
	children := make([]layout.FlexChild, 0, len(rows)*2+1)
	children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
		title := material.Caption(h.th, strings.ToUpper(h.l.T("settings.title")))
		title.Color = theme.TextSecondary
		title.Font.Weight = font.Medium
		title.TextSize = unit.Sp(11)
		return layout.Inset{Bottom: unit.Dp(10)}.Layout(gtx, title.Layout)
	}))
	for i, r := range rows {
		r := r
		if i > 0 {
			children = append(children, layout.Rigid(layout.Spacer{Height: unit.Dp(12)}.Layout))
		}
		children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return h.checkbox(gtx, r.b, r.label)
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					lbl := material.Caption(h.th, r.help)
					lbl.Color = theme.HelpText
					lbl.TextSize = unit.Sp(11)
					return layout.Inset{Left: unit.Dp(28), Top: unit.Dp(2)}.Layout(gtx, lbl.Layout)
				}),
			)
		}))
	}

	macro := op.Record(gtx.Ops)
	dims := layout.UniformInset(unit.Dp(16)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
	})
	content := macro.Stop()
	paint.FillShape(gtx.Ops, theme.Background, clip.Rect{Max: dims.Size}.Op())
	drawBorderAll(gtx, dims.Size, theme.Divider)
	content.Add(gtx.Ops)
	return dims
}

func (h *Home) languageDropdown() layout.Widget {
	langs := h.l.Languages()
	opts := make([]ddOption, len(langs))
	for i, lg := range langs {
		opts[i] = ddOption{value: lg.Code, label: lg.Label, hint: lg.Short}
	}
	current := h.l.Lang()
	var currentShort string
	for _, lg := range langs {
		if lg.Code == current {
			currentShort = lg.Short
			break
		}
	}

	return func(gtx layout.Context) layout.Dimensions {
		for len(h.langDD.items) < len(opts) {
			h.langDD.items = append(h.langDD.items, widget.Clickable{})
		}
		if h.langDD.trigger.Clicked(gtx) {
			h.langDD.open = !h.langDD.open
		}
		for i := range h.langDD.items {
			if h.langDD.items[i].Clicked(gtx) && i < len(opts) {
				h.applyLanguage(opts[i].value)
				h.langDD.open = false
			}
		}

		dims := h.langDD.trigger.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			hovered := h.langDD.trigger.Hovered() || h.langDD.open
			macro := op.Record(gtx.Ops)
			d := h.langTriggerContent(gtx, currentShort)
			content := macro.Stop()
			if hovered {
				paint.FillShape(gtx.Ops, hoverColor(), clip.Rect{Max: d.Size}.Op())
			}
			content.Add(gtx.Ops)
			return d
		})

		if h.langDD.open {
			// Each item ≈ 34dp + 2dp border. Estimate the menu height
			// to place it just above the trigger without overlapping.
			itemH := gtx.Dp(unit.Dp(34))
			menuH := itemH*len(opts) + gtx.Dp(unit.Dp(2))
			minW := gtx.Dp(unit.Dp(160))
			if minW < dims.Size.X {
				minW = dims.Size.X
			}
			offset := image.Pt(0, -menuH-2)
			h.deferDropdownMenu(gtx, &h.langDD, opts, current, offset, minW)
		}
		return dims
	}
}

func (h *Home) langTriggerContent(gtx layout.Context, short string) layout.Dimensions {
	return layout.Inset{Left: unit.Dp(8), Right: unit.Dp(8), Top: unit.Dp(6), Bottom: unit.Dp(6)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions { return h.drawGlobe(gtx, theme.TextMuted) }),
			layout.Rigid(layout.Spacer{Width: unit.Dp(6)}.Layout),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				lbl := material.Caption(h.th, short)
				lbl.Color = theme.TextSecondary
				lbl.TextSize = unit.Sp(12)
				return lbl.Layout(gtx)
			}),
			layout.Rigid(layout.Spacer{Width: unit.Dp(4)}.Layout),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions { return h.drawChevron(gtx, theme.TextMuted) }),
		)
	})
}

// ───────── Action bar buttons ─────────

func (h *Home) iconBtn(clk *widget.Clickable, draw func(layout.Context, color.NRGBA) layout.Dimensions, accent bool) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		return clk.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			macro := op.Record(gtx.Ops)
			dims := layout.UniformInset(unit.Dp(8)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				col := theme.TextSecondary
				if accent {
					col = theme.Accent
				}
				return draw(gtx, col)
			})
			content := macro.Stop()
			if clk.Hovered() {
				paint.FillShape(gtx.Ops, hoverColor(), clip.Rect{Max: dims.Size}.Op())
			}
			content.Add(gtx.Ops)
			return dims
		})
	}
}

func (h *Home) ghostButton(clk *widget.Clickable, label string) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		return clk.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			macro := op.Record(gtx.Ops)
			dims := layout.Inset{Top: unit.Dp(6), Bottom: unit.Dp(6), Left: unit.Dp(14), Right: unit.Dp(14)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				lbl := material.Body2(h.th, label)
				lbl.TextSize = unit.Sp(12)
				if clk.Hovered() {
					lbl.Color = theme.TextPrimary
				} else {
					lbl.Color = theme.TextSecondary
				}
				return lbl.Layout(gtx)
			})
			content := macro.Stop()
			if clk.Hovered() {
				paint.FillShape(gtx.Ops, hoverColor(), clip.Rect{Max: dims.Size}.Op())
			}
			content.Add(gtx.Ops)
			return dims
		})
	}
}

func (h *Home) outlineButton(clk *widget.Clickable, label string, disabled bool) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		return clk.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			macro := op.Record(gtx.Ops)
			dims := layout.Inset{Top: unit.Dp(6), Bottom: unit.Dp(6), Left: unit.Dp(14), Right: unit.Dp(14)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				lbl := material.Body2(h.th, label)
				lbl.TextSize = unit.Sp(12)
				if disabled {
					lbl.Color = withAlpha(theme.Accent, 0x60)
				} else if clk.Hovered() {
					lbl.Color = theme.Background
				} else {
					lbl.Color = theme.Accent
				}
				return lbl.Layout(gtx)
			})
			content := macro.Stop()
			if !disabled && clk.Hovered() {
				paint.FillShape(gtx.Ops, theme.Accent, clip.Rect{Max: dims.Size}.Op())
			}
			drawBorderAll(gtx, dims.Size, theme.InputLine)
			content.Add(gtx.Ops)
			return dims
		})
	}
}

func (h *Home) primaryButton(clk *widget.Clickable, label string, disabled bool) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		return clk.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			macro := op.Record(gtx.Ops)
			dims := layout.Inset{Top: unit.Dp(6), Bottom: unit.Dp(6), Left: unit.Dp(14), Right: unit.Dp(14)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				lbl := material.Body2(h.th, label)
				lbl.TextSize = unit.Sp(12)
				lbl.Color = theme.Background
				if disabled {
					lbl.Color = withAlpha(theme.Background, 0x80)
				}
				return lbl.Layout(gtx)
			})
			content := macro.Stop()
			bg := theme.Accent
			if disabled {
				bg = withAlpha(theme.Accent, 0x55)
			} else if clk.Hovered() {
				bg = theme.TextPrimary
			}
			paint.FillShape(gtx.Ops, bg, clip.Rect{Max: dims.Size}.Op())
			content.Add(gtx.Ops)
			return dims
		})
	}
}

// ───────── Icons (Gio paint) ─────────

func (h *Home) drawHamburger(gtx layout.Context, c color.NRGBA) layout.Dimensions {
	s := gtx.Dp(unit.Dp(14))
	line := gtx.Dp(unit.Dp(1))
	for i := 0; i < 3; i++ {
		y := i * (s/3 + line/2)
		w := s
		if i == 2 {
			w = s * 6 / 8
		}
		paint.FillShape(gtx.Ops, c, clip.Rect{Min: image.Pt(0, y), Max: image.Pt(w, y+line)}.Op())
	}
	return layout.Dimensions{Size: image.Pt(s, s)}
}

func (h *Home) drawTrash(gtx layout.Context, c color.NRGBA) layout.Dimensions {
	s := gtx.Dp(unit.Dp(12))
	line := gtx.Dp(unit.Dp(1))
	// Lid.
	paint.FillShape(gtx.Ops, c, clip.Rect{Min: image.Pt(s/8, s*3/12), Max: image.Pt(s-s/8, s*3/12+line)}.Op())
	// Body (3 vertical lines).
	for _, x := range []int{s * 3 / 12, s / 2, s * 9 / 12} {
		paint.FillShape(gtx.Ops, c, clip.Rect{Min: image.Pt(x, s*4/12), Max: image.Pt(x+line, s)}.Op())
	}
	return layout.Dimensions{Size: image.Pt(s, s)}
}

// drawGear renders a minimal gear: a thin ring with a center hole and
// eight short teeth, in keeping with the lineal wabi-sabi iconography.
func (h *Home) drawGear(gtx layout.Context, c color.NRGBA) layout.Dimensions {
	s := gtx.Dp(unit.Dp(15))
	cx, cy := float32(s)/2, float32(s)/2

	rOut := float32(gtx.Dp(unit.Dp(4.4)))
	rIn := float32(gtx.Dp(unit.Dp(2.6)))

	toothLen := float32(gtx.Dp(unit.Dp(2.2)))
	toothThick := gtx.Dp(unit.Dp(1.8))
	if toothThick < 1 {
		toothThick = 1
	}
	for i := 0; i < 8; i++ {
		angle := float32(i) * math.Pi / 4
		tr := op.Affine(f32.Affine2D{}.Rotate(f32.Pt(cx, cy), angle)).Push(gtx.Ops)
		rect := clip.Rect{
			Min: image.Pt(int(cx+rOut)-1, int(cy)-toothThick/2),
			Max: image.Pt(int(cx+rOut+toothLen), int(cy)+toothThick/2+1),
		}
		paint.FillShape(gtx.Ops, c, rect.Op())
		tr.Pop()
	}

	// Ring body as an annulus via the even/odd fill rule.
	var p clip.Path
	p.Begin(gtx.Ops)
	circle(&p, f32.Pt(cx, cy), rOut, false)
	circle(&p, f32.Pt(cx, cy), rIn, true)
	paint.FillShape(gtx.Ops, c, clip.Outline{Path: p.End()}.Op())

	return layout.Dimensions{Size: image.Pt(s, s)}
}

func (h *Home) drawChevron(gtx layout.Context, c color.NRGBA) layout.Dimensions {
	s := gtx.Dp(unit.Dp(10))
	line := gtx.Dp(unit.Dp(1))
	mid := s / 2
	// Two strokes forming a V.
	for i := 0; i < mid; i++ {
		paint.FillShape(gtx.Ops, c, clip.Rect{Min: image.Pt(mid-i, 2+i), Max: image.Pt(mid-i+line, 2+i+line)}.Op())
		paint.FillShape(gtx.Ops, c, clip.Rect{Min: image.Pt(mid+i, 2+i), Max: image.Pt(mid+i+line, 2+i+line)}.Op())
	}
	return layout.Dimensions{Size: image.Pt(s, s)}
}

func (h *Home) drawGlobe(gtx layout.Context, c color.NRGBA) layout.Dimensions {
	s := gtx.Dp(unit.Dp(13))
	line := gtx.Dp(unit.Dp(1))
	// Ring approximated as a foreground disc with a chrome-colored
	// inner disc on top.
	paint.FillShape(gtx.Ops, c, clip.Ellipse{Max: image.Pt(s, s)}.Op(gtx.Ops))
	paint.FillShape(gtx.Ops, theme.Chrome, clip.Ellipse{Min: image.Pt(line+1, line+1), Max: image.Pt(s-line-1, s-line-1)}.Op(gtx.Ops))
	// Equator.
	paint.FillShape(gtx.Ops, c, clip.Rect{Min: image.Pt(0, s/2), Max: image.Pt(s, s/2+line)}.Op())
	// Meridian.
	paint.FillShape(gtx.Ops, c, clip.Rect{Min: image.Pt(s/2, 0), Max: image.Pt(s/2+line, s)}.Op())
	return layout.Dimensions{Size: image.Pt(s, s)}
}

// drawSun renders a minimal sun: central disc plus 8 short rays
// rotated with op.Affine.
func (h *Home) drawSun(gtx layout.Context, c color.NRGBA) layout.Dimensions {
	s := gtx.Dp(unit.Dp(15))
	cx, cy := float32(s)/2, float32(s)/2
	// Central disc.
	disc := gtx.Dp(unit.Dp(3))
	paint.FillShape(gtx.Ops, c, clip.Ellipse{
		Min: image.Pt(int(cx)-disc, int(cy)-disc),
		Max: image.Pt(int(cx)+disc, int(cy)+disc),
	}.Op(gtx.Ops))

	rayLen := float32(gtx.Dp(unit.Dp(2.5)))
	rayThick := float32(gtx.Dp(unit.Dp(1.4)))
	if rayThick < 1 {
		rayThick = 1
	}
	rayInner := float32(disc) + float32(gtx.Dp(unit.Dp(1.5)))

	// Draw one ray to the right of center and rotate it 8 times.
	for i := 0; i < 8; i++ {
		angle := float32(i) * math.Pi / 4
		tr := op.Affine(f32.Affine2D{}.Rotate(f32.Pt(cx, cy), angle)).Push(gtx.Ops)
		rect := clip.Rect{
			Min: image.Pt(int(cx+rayInner), int(cy-rayThick/2)),
			Max: image.Pt(int(cx+rayInner+rayLen), int(cy+rayThick/2+1)),
		}
		paint.FillShape(gtx.Ops, c, rect.Op())
		tr.Pop()
	}
	return layout.Dimensions{Size: image.Pt(s, s)}
}

// drawMoon renders a crescent as a single clip.Path: outer circle plus
// a reversed inner circle, combined via the even/odd fill rule.
func (h *Home) drawMoon(gtx layout.Context, c color.NRGBA) layout.Dimensions {
	s := gtx.Dp(unit.Dp(15))
	cx, cy := float32(s)/2, float32(s)/2
	rOut := float32(gtx.Dp(unit.Dp(6.5)))
	rIn := float32(gtx.Dp(unit.Dp(5.5)))
	offX := float32(gtx.Dp(unit.Dp(2.8)))
	offY := -float32(gtx.Dp(unit.Dp(0.6)))

	var p clip.Path
	p.Begin(gtx.Ops)
	// Outer circle, clockwise, approximated with four cubic arcs.
	circle(&p, f32.Pt(cx, cy), rOut, false)
	// Clipping circle, counter-clockwise; even/odd fill subtracts it.
	circle(&p, f32.Pt(cx+offX, cy+offY), rIn, true)
	paint.FillShape(gtx.Ops, c, clip.Outline{Path: p.End()}.Op())
	return layout.Dimensions{Size: image.Pt(s, s)}
}

// circle traces a circle on the path as four cubic Bezier curves.
// reverse flips direction so even/odd treats it as a hole when combined
// with a previous outline.
func circle(p *clip.Path, c f32.Point, r float32, reverse bool) {
	// kappa ≈ 0.5522847 — control-point offset that approximates a
	// quarter circle with one cubic Bezier.
	const k = 0.5522847498307936
	d := r * k
	if !reverse {
		p.MoveTo(f32.Pt(c.X+r, c.Y))
		p.CubeTo(f32.Pt(c.X+r, c.Y+d), f32.Pt(c.X+d, c.Y+r), f32.Pt(c.X, c.Y+r))
		p.CubeTo(f32.Pt(c.X-d, c.Y+r), f32.Pt(c.X-r, c.Y+d), f32.Pt(c.X-r, c.Y))
		p.CubeTo(f32.Pt(c.X-r, c.Y-d), f32.Pt(c.X-d, c.Y-r), f32.Pt(c.X, c.Y-r))
		p.CubeTo(f32.Pt(c.X+d, c.Y-r), f32.Pt(c.X+r, c.Y-d), f32.Pt(c.X+r, c.Y))
	} else {
		p.MoveTo(f32.Pt(c.X+r, c.Y))
		p.CubeTo(f32.Pt(c.X+r, c.Y-d), f32.Pt(c.X+d, c.Y-r), f32.Pt(c.X, c.Y-r))
		p.CubeTo(f32.Pt(c.X-d, c.Y-r), f32.Pt(c.X-r, c.Y-d), f32.Pt(c.X-r, c.Y))
		p.CubeTo(f32.Pt(c.X-r, c.Y+d), f32.Pt(c.X-d, c.Y+r), f32.Pt(c.X, c.Y+r))
		p.CubeTo(f32.Pt(c.X+d, c.Y+r), f32.Pt(c.X+r, c.Y+d), f32.Pt(c.X+r, c.Y))
	}
	p.Close()
}

// ───────── Paint helpers ─────────

func verticalDivider(gtx layout.Context) layout.Dimensions {
	w := gtx.Dp(unit.Dp(1))
	paint.FillShape(gtx.Ops, theme.Divider, clip.Rect{Max: image.Pt(w, gtx.Constraints.Max.Y)}.Op())
	return layout.Dimensions{Size: image.Pt(w, gtx.Constraints.Max.Y)}
}

func (h *Home) dividerHorizontal(gtx layout.Context) layout.Dimensions {
	w := gtx.Constraints.Max.X
	hg := gtx.Dp(unit.Dp(1))
	paint.FillShape(gtx.Ops, theme.Divider, clip.Rect{Max: image.Pt(w, hg)}.Op())
	return layout.Dimensions{Size: image.Pt(w, hg)}
}

func drawTopBorder(gtx layout.Context, size image.Point, c color.NRGBA) {
	hg := gtx.Dp(unit.Dp(1))
	paint.FillShape(gtx.Ops, c, clip.Rect{Max: image.Pt(size.X, hg)}.Op())
}

func drawBottomBorder(gtx layout.Context, size image.Point, c color.NRGBA) {
	hg := gtx.Dp(unit.Dp(1))
	defer op.Offset(image.Pt(0, size.Y-hg)).Push(gtx.Ops).Pop()
	paint.FillShape(gtx.Ops, c, clip.Rect{Max: image.Pt(size.X, hg)}.Op())
}

func drawBorderAll(gtx layout.Context, size image.Point, c color.NRGBA) {
	hg := gtx.Dp(unit.Dp(1))
	paint.FillShape(gtx.Ops, c, clip.Rect{Max: image.Pt(size.X, hg)}.Op())
	paint.FillShape(gtx.Ops, c, clip.Rect{Min: image.Pt(0, size.Y-hg), Max: image.Pt(size.X, size.Y)}.Op())
	paint.FillShape(gtx.Ops, c, clip.Rect{Max: image.Pt(hg, size.Y)}.Op())
	paint.FillShape(gtx.Ops, c, clip.Rect{Min: image.Pt(size.X-hg, 0), Max: image.Pt(size.X, size.Y)}.Op())
}

func hoverColor() color.NRGBA {
	return withAlpha(theme.Accent, 0x12)
}

func withAlpha(c color.NRGBA, a uint8) color.NRGBA {
	return color.NRGBA{R: c.R, G: c.G, B: c.B, A: a}
}

// ───────── Error banner ─────────

// humanizeError maps a preset.Validate error to a localized,
// user-facing message. The validator returns log-oriented strings; this
// function translates them into something a non-technical user can act on.
func (h *Home) humanizeError(err error) string {
	if err == nil {
		return ""
	}
	msg := err.Error()
	// Substring matching keeps the validator free of typed error codes.
	switch {
	case containsAny(msg, "details needs"):
		return h.l.T("error.details_min")
	case containsAny(msg, "state needs"):
		return h.l.T("error.state_min")
	case containsAny(msg, "client_id is required"):
		return h.l.T("error.client_id")
	case containsAny(msg, "name length must be"):
		return h.l.T("error.name")
	case containsAny(msg, "must be http(s)"):
		return h.l.T("error.url_invalid")
	case containsAny(msg, "must include a host"):
		return h.l.T("error.url_host")
	case containsAny(msg, "requires both label and url"):
		return h.l.T("error.button")
	case containsAny(msg, "invalid party size"):
		return h.l.T("error.party")
	case containsAny(msg, "unknown activity type"):
		return h.l.T("error.type")
	case containsAny(msg, "unknown display field"):
		return h.l.T("error.display")
	case containsAny(msg, "unknown time mode"):
		return h.l.T("error.time")
	case containsAny(msg, "> ", "chars"):
		return h.l.T("error.text_too_long")
	}
	return h.l.T("error.generic", msg)
}

func containsAny(s string, subs ...string) bool {
	for _, sub := range subs {
		if !strings.Contains(s, sub) {
			return false
		}
	}
	return true
}

// errorBannerWidget paints a terracotta strip under the topbar with the
// active message and an X to dismiss. Renders empty (zero size) when no
// error is active.
func (h *Home) errorBannerWidget(gtx layout.Context) layout.Dimensions {
	if h.errorBanner == "" {
		return layout.Dimensions{}
	}
	macro := op.Record(gtx.Ops)
	dims := layout.Inset{Top: unit.Dp(10), Bottom: unit.Dp(10), Left: unit.Dp(16), Right: unit.Dp(12)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				title := material.Body2(h.th, h.l.T("error.banner.title"))
				title.Color = theme.StatusError
				title.Font.Weight = font.Medium
				title.TextSize = unit.Sp(12)
				return title.Layout(gtx)
			}),
			layout.Rigid(layout.Spacer{Width: unit.Dp(12)}.Layout),
			layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
				msg := material.Body2(h.th, h.errorBanner)
				msg.Color = theme.TextPrimary
				msg.TextSize = unit.Sp(12)
				return msg.Layout(gtx)
			}),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return h.errorDismissed.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					inner := layout.UniformInset(unit.Dp(4)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						c := theme.TextMuted
						if h.errorDismissed.Hovered() {
							c = theme.TextPrimary
						}
						return h.drawX(gtx, c)
					})
					return inner
				})
			}),
		)
	})
	content := macro.Stop()
	// Very subtle StatusError tint, preserves the overall palette.
	paint.FillShape(gtx.Ops, withAlpha(theme.StatusError, 0x18), clip.Rect{Max: dims.Size}.Op())
	drawBottomBorder(gtx, dims.Size, theme.Divider)
	content.Add(gtx.Ops)
	return dims
}

// updateBannerWidget paints a subtle accent strip under the topbar when a
// newer release is available, with a "view release" link that opens the
// browser and an X to dismiss for this session. Renders empty otherwise.
func (h *Home) updateBannerWidget(gtx layout.Context) layout.Dimensions {
	rel, ok := h.updateAvailable()
	if !ok || h.updateHidden {
		return layout.Dimensions{}
	}
	macro := op.Record(gtx.Ops)
	dims := layout.Inset{Top: unit.Dp(10), Bottom: unit.Dp(10), Left: unit.Dp(16), Right: unit.Dp(12)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
			layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
				msg := material.Body2(h.th, h.l.T("update.banner", rel.Tag))
				msg.Color = theme.TextPrimary
				msg.TextSize = unit.Sp(12)
				return msg.Layout(gtx)
			}),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return h.updateLink.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					return layout.Inset{Left: unit.Dp(8), Right: unit.Dp(8), Top: unit.Dp(2), Bottom: unit.Dp(2)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						lbl := material.Body2(h.th, h.l.T("update.view"))
						lbl.Color = theme.Accent
						lbl.Font.Weight = font.Medium
						lbl.TextSize = unit.Sp(12)
						return lbl.Layout(gtx)
					})
				})
			}),
			layout.Rigid(layout.Spacer{Width: unit.Dp(4)}.Layout),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return h.updateDismissed.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					return layout.UniformInset(unit.Dp(4)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						c := theme.TextMuted
						if h.updateDismissed.Hovered() {
							c = theme.TextPrimary
						}
						return h.drawX(gtx, c)
					})
				})
			}),
		)
	})
	content := macro.Stop()
	paint.FillShape(gtx.Ops, withAlpha(theme.Accent, 0x14), clip.Rect{Max: dims.Size}.Op())
	drawBottomBorder(gtx, dims.Size, theme.Divider)
	content.Add(gtx.Ops)
	return dims
}

// drawX draws a minimal X glyph as two rotated rectangles via op.Affine.
func (h *Home) drawX(gtx layout.Context, c color.NRGBA) layout.Dimensions {
	s := gtx.Dp(unit.Dp(12))
	cx, cy := float32(s)/2, float32(s)/2
	thick := gtx.Dp(unit.Dp(1.2))
	if thick < 1 {
		thick = 1
	}
	half := float32(s) * 0.35
	for _, angle := range []float64{math.Pi / 4, -math.Pi / 4} {
		tr := op.Affine(f32.Affine2D{}.Rotate(f32.Pt(cx, cy), float32(angle))).Push(gtx.Ops)
		paint.FillShape(gtx.Ops, c, clip.Rect{
			Min: image.Pt(int(cx-half), int(cy)-thick/2),
			Max: image.Pt(int(cx+half), int(cy)+thick/2+1),
		}.Op())
		tr.Pop()
	}
	return layout.Dimensions{Size: image.Pt(s, s)}
}

// ───────── Click handling ─────────

func (h *Home) handleClicks(gtx layout.Context) {
	if h.sidebarBtn.Clicked(gtx) {
		_ = config.Apply(h.cfgDir, h.cfg, func(c *config.Config) { c.SidebarOpen = !c.SidebarOpen })
	}
	if h.newBtn.Clicked(gtx) {
		h.createPreset()
	}
	list := h.mgr.List()
	for i := range h.presetClicks {
		if i < len(list) && h.presetClicks[i].Clicked(gtx) {
			h.loadInto(list[i])
		}
	}
	for i := range h.presetDeletes {
		if i < len(list) && h.presetDeletes[i].Clicked(gtx) {
			_ = h.mgr.Delete(list[i].ID)
			if h.editingID == list[i].ID {
				h.resetForm()
			}
		}
	}
	if h.saveBtn.Clicked(gtx) {
		h.savePreset()
	}
	if h.connectBtn.Clicked(gtx) {
		if h.statusText == "connected" {
			h.ctrl.Disconnect()
		} else if h.statusText != "connecting" {
			h.applyForm()
		}
	}
	if h.updateBtn.Clicked(gtx) && h.statusText == "connected" {
		if err := h.ctrl.Apply(h.formToPreset()); err != nil {
			log.Printf("home: update presence: %v", err)
			h.errorBanner = h.humanizeError(err)
		} else {
			h.updateFlash = time.Now()
			h.errorBanner = ""
		}
	}
	if h.errorDismissed.Clicked(gtx) {
		h.errorBanner = ""
	}
	if h.settingsBtn.Clicked(gtx) {
		h.settingsOpen = !h.settingsOpen
	}
	// Settings toggles. Update() is called here (before the deferred panel
	// renders) so the click is consumed once and the side effect fires.
	if h.autoStart.Update(gtx) {
		if err := platform.SetAutoStart(h.autoStart.Value); err != nil {
			log.Printf("home: set autostart: %v", err)
		}
		_ = config.Apply(h.cfgDir, h.cfg, func(c *config.Config) { c.StartWithSystem = h.autoStart.Value })
	}
	if h.startMin.Update(gtx) {
		_ = config.Apply(h.cfgDir, h.cfg, func(c *config.Config) { c.StartMinimized = h.startMin.Value })
	}
	if h.checkUpd.Update(gtx) {
		_ = config.Apply(h.cfgDir, h.cfg, func(c *config.Config) { c.CheckUpdates = h.checkUpd.Value })
	}
	if h.updateLink.Clicked(gtx) {
		if rel, ok := h.updateAvailable(); ok && rel.URL != "" {
			if err := platform.OpenURL(rel.URL); err != nil {
				log.Printf("home: open release page: %v", err)
			}
		}
	}
	if h.updateDismissed.Clicked(gtx) {
		h.updateHidden = true
	}
	if h.themeBtn.Clicked(gtx) {
		h.toggleTheme()
	}
}

func (h *Home) toggleTheme() {
	next := "dark"
	if theme.Mode() == "dark" {
		next = "light"
	}
	theme.SetMode(next)
	_ = config.Apply(h.cfgDir, h.cfg, func(c *config.Config) { c.Theme = next })
	if h.OnThemeChange != nil {
		h.OnThemeChange()
	}
}

func (h *Home) applyLanguage(code string) {
	if code == h.l.Lang() {
		return
	}
	if err := h.l.SetLanguage(code); err != nil {
		log.Printf("home: SetLanguage: %v", err)
		return
	}
	_ = config.Apply(h.cfgDir, h.cfg, func(c *config.Config) { c.Language = code })
}

func (h *Home) savePreset() {
	p := h.formToPreset()
	if err := h.mgr.Upsert(p); err != nil {
		log.Printf("home: save preset: %v", err)
		h.errorBanner = h.humanizeError(err)
		return
	}
	h.editingID = p.ID
	h.saveFlash = time.Now()
	h.errorBanner = ""
}

func (h *Home) applyForm() {
	p := h.formToPreset()
	if err := h.ctrl.Apply(p); err != nil {
		log.Printf("home: apply: %v", err)
		h.errorBanner = h.humanizeError(err)
		return
	}
	h.errorBanner = ""
}

func (h *Home) createPreset() {
	h.resetForm()
	h.presetName.SetText(h.l.T("sidebar.new"))
	h.typeEnum.Value = string(preset.TypePlaying)
	h.dispEnum.Value = string(preset.DisplayName)
	h.timeEnum.Value = string(preset.TimeNone)
}

func (h *Home) resetForm() {
	h.editingID = ""
	editors := []*widget.Editor{
		&h.presetName, &h.clientID, &h.appName, &h.details, &h.detailsURL,
		&h.state, &h.stateURL, &h.partyCur, &h.partyMax,
		&h.timeStart, &h.timeEnd,
		&h.largeKey, &h.largeText, &h.largeURL,
		&h.smallKey, &h.smallText, &h.smallURL,
		&h.btn1Text, &h.btn1URL, &h.btn2Text, &h.btn2URL,
	}
	for _, e := range editors {
		e.SetText("")
	}
	h.timeEndOn.Value = false
}

func (h *Home) loadInto(p *preset.Preset) {
	h.editingID = p.ID
	h.presetName.SetText(p.Name)
	h.clientID.SetText(p.ClientID)
	h.appName.SetText(p.AppName)
	h.details.SetText(p.Details)
	h.detailsURL.SetText(p.DetailsURL)
	h.state.SetText(p.State)
	h.stateURL.SetText(p.StateURL)
	h.partyCur.SetText(strconv.Itoa(p.PartySize))
	h.partyMax.SetText(strconv.Itoa(p.PartyMax))
	h.timeStart.SetText(p.TimeStart)
	h.timeEnd.SetText(p.TimeEnd)
	h.timeEndOn.Value = p.TimeEndOn
	h.largeKey.SetText(p.LargeKey)
	h.largeText.SetText(p.LargeText)
	h.largeURL.SetText(p.LargeURL)
	h.smallKey.SetText(p.SmallKey)
	h.smallText.SetText(p.SmallText)
	h.smallURL.SetText(p.SmallURL)
	h.btn1Text.SetText(p.Btn1Text)
	h.btn1URL.SetText(p.Btn1URL)
	h.btn2Text.SetText(p.Btn2Text)
	h.btn2URL.SetText(p.Btn2URL)
	if p.Type != "" {
		h.typeEnum.Value = string(p.Type)
	} else {
		h.typeEnum.Value = string(preset.TypePlaying)
	}
	if p.Display != "" {
		h.dispEnum.Value = string(p.Display)
	} else {
		h.dispEnum.Value = string(preset.DisplayName)
	}
	if p.TimeMode != "" {
		h.timeEnum.Value = string(p.TimeMode)
	} else {
		h.timeEnum.Value = string(preset.TimeNone)
	}
}

func (h *Home) formToPreset() *preset.Preset {
	parseInt := func(ed *widget.Editor) int {
		v, _ := strconv.Atoi(strings.TrimSpace(ed.Text()))
		if v < 0 {
			v = 0
		}
		return v
	}
	return &preset.Preset{
		ID:         h.editingID,
		Name:       strings.TrimSpace(h.presetName.Text()),
		ClientID:   strings.TrimSpace(h.clientID.Text()),
		Type:       preset.ActivityType(h.typeEnum.Value),
		Display:    preset.DisplayField(h.dispEnum.Value),
		AppName:    strings.TrimSpace(h.appName.Text()),
		Details:    strings.TrimSpace(h.details.Text()),
		DetailsURL: strings.TrimSpace(h.detailsURL.Text()),
		State:      strings.TrimSpace(h.state.Text()),
		StateURL:   strings.TrimSpace(h.stateURL.Text()),
		PartySize:  parseInt(&h.partyCur),
		PartyMax:   parseInt(&h.partyMax),
		TimeMode:   preset.TimeMode(h.timeEnum.Value),
		TimeStart:  strings.TrimSpace(h.timeStart.Text()),
		TimeEndOn:  h.timeEndOn.Value,
		TimeEnd:    strings.TrimSpace(h.timeEnd.Text()),
		LargeKey:   strings.TrimSpace(h.largeKey.Text()),
		LargeText:  strings.TrimSpace(h.largeText.Text()),
		LargeURL:   strings.TrimSpace(h.largeURL.Text()),
		SmallKey:   strings.TrimSpace(h.smallKey.Text()),
		SmallText:  strings.TrimSpace(h.smallText.Text()),
		SmallURL:   strings.TrimSpace(h.smallURL.Text()),
		Btn1Text:   strings.TrimSpace(h.btn1Text.Text()),
		Btn1URL:    strings.TrimSpace(h.btn1URL.Text()),
		Btn2Text:   strings.TrimSpace(h.btn2Text.Text()),
		Btn2URL:    strings.TrimSpace(h.btn2URL.Text()),
	}
}
