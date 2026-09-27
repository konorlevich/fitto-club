package content

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"reflect"
	"strings"
)

// SiteCopy is every fixed string on the site, per locale. Templates read
// .Copy.Home.HeroWord and never branch on locale (checklist §5). Display
// headings are stored already in capitals for en/ru, because
// text-transform is banned for Cyrillic and meaningless for Georgian.
type SiteCopy struct {
	Common      CommonCopy      `json:"common"`
	Nav         NavCopy         `json:"nav"`
	Visit       VisitCopy       `json:"visit"`
	Home        HomeCopy        `json:"home"`
	Memberships MembershipsCopy `json:"memberships"`
	Coaches     CoachesCopy     `json:"coaches"`
	Coach       CoachCopy       `json:"coach"`
	Classes     ClassesCopy     `json:"classes"`
	Massage     MassageCopy     `json:"massage"`
	About       AboutCopy       `json:"about"`
	Reviews     ReviewsCopy     `json:"reviews"`
	Privacy     PrivacyCopy     `json:"privacy"`
	NotFound    NotFoundCopy    `json:"notfound"`
	Consent     ConsentCopy     `json:"consent"`
}

type CommonCopy struct {
	SiteName       string   `json:"site_name"`
	Tagline        string   `json:"tagline"`
	SkipToContent  string   `json:"skip_to_content"`
	FirstVisit     string   `json:"first_visit"`
	Call           string   `json:"call"`
	Message        string   `json:"message"`
	InstagramLabel string   `json:"instagram_label"`
	OpensInNewTab  string   `json:"opens_in_new_tab"`
	LanguageLabel  string   `json:"language_label"`
	Currency       string   `json:"currency"`
	From           string   `json:"from"` // format: "from %s"
	PerMonthShort  string   `json:"per_month_short"`
	Approx         string   `json:"approx"`
	Minutes        string   `json:"minutes"`
	OpenUntil      string   `json:"open_until"`
	ClosedOpensAt  string   `json:"closed_opens_at"`
	ClosedOpensDay string   `json:"closed_opens_day"`
	Tomorrow       string   `json:"tomorrow"`
	Today          string   `json:"today"`
	Days           []string `json:"days"`
	DaysShort      []string `json:"days_short"`
	DaysOn         []string `json:"days_on"`
	Closed         string   `json:"closed"`
	RatingLine     string   `json:"rating_line"`
	StarsOf        string   `json:"stars_of"`
	LangNames      []string `json:"lang_names"`
	LangShort      []string `json:"lang_short"`
	CreatedBy      string   `json:"created_by"`
	PrivacyLink    string   `json:"privacy_link"`
	CookieSettings string   `json:"cookie_settings"`
	LegalEntity    string   `json:"legal_entity"`
	FooterNote     string   `json:"footer_note"`
	HoursHeading   string   `json:"hours_heading"`
	ContactHeading string   `json:"contact_heading"`
	PagesHeading   string   `json:"pages_heading"`
}

type NavCopy struct {
	Home        string `json:"home"`
	Memberships string `json:"memberships"`
	Coaches     string `json:"coaches"`
	Classes     string `json:"classes"`
	Massage     string `json:"massage"`
	About       string `json:"about"`
	Menu        string `json:"menu"`
	Close       string `json:"close"`
	Primary     string `json:"primary"`
	BarLabel    string `json:"bar_label"`
}

type VisitCopy struct {
	Heading     string   `json:"heading"`
	Lede        string   `json:"lede"`
	AddressHead string   `json:"address_head"`
	Landmarks   string   `json:"landmarks"`
	Google      string   `json:"google"`
	Apple       string   `json:"apple"`
	MapAlt      string   `json:"map_alt"`
	MapCredit   string   `json:"map_credit"`
	VideoHead   string   `json:"video_head"`
	VideoButton string   `json:"video_button"`
	VideoNote   string   `json:"video_note"`
	Steps       []string `json:"steps"`
	PosterAlt   string   `json:"poster_alt"`
	QuestionsHd string   `json:"questions_heading"`
}

type Step struct {
	Title string `json:"title"`
	Text  string `json:"text"`
}

type HomeCopy struct {
	Title          string `json:"title"`
	Description    string `json:"description"`
	Eyebrow        string `json:"eyebrow"`
	HeroLine       string `json:"hero_line"`
	HeroWord       string `json:"hero_word"`
	HeroSub        string `json:"hero_sub"`
	HeroPhotoAlt   string `json:"hero_photo_alt"`
	FirstHeading   string `json:"first_heading"`
	FirstLede      string `json:"first_lede"`
	FirstSteps     []Step `json:"first_steps"`
	FacadeAlt      string `json:"facade_alt"`
	PriceHeading   string `json:"price_heading"`
	PriceText      string `json:"price_text"`
	PriceSingle    string `json:"price_single"`
	PriceLink      string `json:"price_link"`
	CoachesHeading string `json:"coaches_heading"`
	CoachesLede    string `json:"coaches_lede"`
	CoachesAll     string `json:"coaches_all"`
	ZonesHeading   string `json:"zones_heading"`
	ZonesLede      string `json:"zones_lede"`
	ZonesLink      string `json:"zones_link"`
	ExtrasHeading  string `json:"extras_heading"`
	ClassesTeaser  string `json:"classes_teaser"`
	ClassesNext    string `json:"classes_next"`
	ClassesLink    string `json:"classes_link"`
	MassageTeaser  string `json:"massage_teaser"`
	MassageText    string `json:"massage_text"`
	MassageLink    string `json:"massage_link"`
	ReviewsHeading string `json:"reviews_heading"`
	GuestsLine     string `json:"guests_line"`
	GuestsLink     string `json:"guests_link"`
}

type MembershipsCopy struct {
	Title        string   `json:"title"`
	Description  string   `json:"description"`
	Heading      string   `json:"heading"`
	Lede         string   `json:"lede"`
	FreeHeading  string   `json:"free_heading"`
	FreeText     string   `json:"free_text"`
	SingleLabel  string   `json:"single_label"`
	SingleText   string   `json:"single_text"`
	TypesHeading string   `json:"types_heading"`
	TermLabels   []string `json:"term_labels"`
	Freeze       string   `json:"freeze"`
	NoFreeze     string   `json:"no_freeze"`
	PerMonth     string   `json:"per_month"`
	GiftPT       string   `json:"gift_pt"`
	TermCol      string   `json:"term_col"`
	PriceCol     string   `json:"price_col"`
	IncHeading   string   `json:"included_heading"`
	Included     []string `json:"included"`
	PTHeading    string   `json:"pt_heading"`
	PTText       string   `json:"pt_text"`
	PTLink       string   `json:"pt_link"`
}

type CoachesCopy struct {
	Title       string `json:"title"`
	Description string `json:"description"`
	Heading     string `json:"heading"`
	Lede        string `json:"lede"`
	FilterFocus string `json:"filter_focus"`
	FilterLang  string `json:"filter_lang"`
	FilterNote  string `json:"filter_note"`
	All         string `json:"all"`
	Reset       string `json:"reset"`
	EmptyJoke   string `json:"empty_joke"`
	ShowAll     string `json:"show_all"`
	ShowSpeaks  string `json:"show_speaks"`
	PTFrom      string `json:"pt_from"`
	PTPrice     string `json:"pt_price"`
	Speaks      string `json:"speaks"`
	Found       string `json:"found"`
	NoPhoto     string `json:"no_photo"`
	PhotoAlt    string `json:"photo_alt"`
}

type CoachCopy struct {
	TitleSuffix  string `json:"title_suffix"`
	PTHeading    string `json:"pt_heading"`
	PerHour      string `json:"per_hour"`
	PriceOnAsk   string `json:"price_on_ask"`
	WriteDM      string `json:"write_dm"`
	HisInstagram string `json:"instagram"`
	AboutHeading string `json:"about_heading"`
	FocusHeading string `json:"focus_heading"`
	ClassesHead  string `json:"classes_heading"`
	ReviewsHead  string `json:"reviews_heading"`
	MoreHeading  string `json:"more_heading"`
	Back         string `json:"back"`
	HowToBook    string `json:"how_to_book"`
}

type ClassesCopy struct {
	Title        string `json:"title"`
	Description  string `json:"description"`
	Heading      string `json:"heading"`
	Lede         string `json:"lede"`
	ScheduleHead string `json:"schedule_heading"`
	TimeCol      string `json:"time_col"`
	NoClasses    string `json:"no_classes"`
	ListHeading  string `json:"list_heading"`
	Coach        string `json:"coach"`
	Langs        string `json:"langs"`
	Price        string `json:"price"`
	Ask          string `json:"ask"`
	When         string `json:"when"`
}

type MassageCopy struct {
	Title        string   `json:"title"`
	Description  string   `json:"description"`
	Heading      string   `json:"heading"`
	Lede         string   `json:"lede"`
	Specialist   string   `json:"specialist"`
	PriceHeading string   `json:"price_heading"`
	ServiceCol   string   `json:"service_col"`
	TimeCol      string   `json:"time_col"`
	PriceCol     string   `json:"price_col"`
	PackCol      string   `json:"pack_col"`
	Pack         string   `json:"pack"`
	KindsHeading string   `json:"kinds_heading"`
	Kinds        []string `json:"kinds"`
	Approach     string   `json:"approach"`
	BookHeading  string   `json:"book_heading"`
	BookText     string   `json:"book_text"`
	RoomAlt      string   `json:"room_alt"`
}

type AboutCopy struct {
	Title         string   `json:"title"`
	Description   string   `json:"description"`
	Heading       string   `json:"heading"`
	Story         []string `json:"story"`
	FounderLink   string   `json:"founder_link"`
	ZonesHeading  string   `json:"zones_heading"`
	EventsHeading string   `json:"events_heading"`
	Events        []Step   `json:"events"`
	PartnersHead  string   `json:"partners_heading"`
	PartnersLede  string   `json:"partners_lede"`
	MemeCaption   string   `json:"meme_caption"`
	MemeAlt       string   `json:"meme_alt"`
	PhotoAlt      string   `json:"photo_alt"`
}

type ReviewsCopy struct {
	Heading    string `json:"heading"`
	More       string `json:"more"`
	InLanguage string `json:"in_language"`
	Original   string `json:"original"`
	Read       string `json:"read"`
}

type PrivacyCopy struct {
	Title       string `json:"title"`
	Description string `json:"description"`
	Heading     string `json:"heading"`
	Updated     string `json:"updated"`
	Sections    []Step `json:"sections"`
}

type NotFoundCopy struct {
	Title       string `json:"title"`
	Description string `json:"description"`
	Heading     string `json:"heading"`
	Joke        string `json:"joke"`
	Home        string `json:"home"`
	Coaches     string `json:"coaches"`
	Route       string `json:"route"`
}

type ConsentCopy struct {
	Label   string `json:"label"`
	Body    string `json:"body"`
	Accept  string `json:"accept"`
	Decline string `json:"decline"`
}

// LoadCopy reads one locale bundle and runs the completeness gate.
func LoadCopy(fsys fs.FS, lang string) (*SiteCopy, error) {
	b, err := fs.ReadFile(fsys, "content/i18n/"+lang+".json")
	if err != nil {
		return nil, fmt.Errorf("locale %s: %w", lang, err)
	}
	var c SiteCopy
	dec := json.NewDecoder(strings.NewReader(string(b)))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&c); err != nil {
		return nil, fmt.Errorf("locale %s: %w", lang, err)
	}
	if missing := missingKeys(reflect.ValueOf(c), ""); len(missing) > 0 {
		return nil, fmt.Errorf("locale %s is incomplete - %d empty value(s): %s",
			lang, len(missing), strings.Join(missing, ", "))
	}
	if err := shapeCheck(&c); err != nil {
		return nil, fmt.Errorf("locale %s: %w", lang, err)
	}
	return &c, nil
}

// shapeCheck guards the lists whose length is structural: a week has seven
// days, the site knows five spoken languages and four membership terms.
func shapeCheck(c *SiteCopy) error {
	for name, n := range map[string][2]int{
		"common.days":             {len(c.Common.Days), 7},
		"common.days_short":       {len(c.Common.DaysShort), 7},
		"common.days_on":          {len(c.Common.DaysOn), 7},
		"common.lang_names":       {len(c.Common.LangNames), len(SpokenLanguages)},
		"common.lang_short":       {len(c.Common.LangShort), len(SpokenLanguages)},
		"memberships.term_labels": {len(c.Memberships.TermLabels), 4},
	} {
		if n[0] != n[1] {
			return fmt.Errorf("%s has %d entries, want %d", name, n[0], n[1])
		}
	}
	return nil
}

// missingKeys reflect-walks the struct and reports every empty string, in
// fields, in string lists and in lists of structs. Any hit is a fatal
// startup error, so a half-translated page can never ship (checklist §5).
func missingKeys(v reflect.Value, prefix string) []string {
	var out []string
	switch v.Kind() {
	case reflect.Struct:
		t := v.Type()
		for i := 0; i < v.NumField(); i++ {
			name := t.Field(i).Tag.Get("json")
			if name == "" {
				name = t.Field(i).Name
			}
			path := name
			if prefix != "" {
				path = prefix + "." + name
			}
			out = append(out, missingKeys(v.Field(i), path)...)
		}
	case reflect.Slice:
		if v.Len() == 0 {
			out = append(out, prefix+"[]")
		}
		for i := 0; i < v.Len(); i++ {
			out = append(out, missingKeys(v.Index(i), fmt.Sprintf("%s[%d]", prefix, i))...)
		}
	case reflect.String:
		if strings.TrimSpace(v.String()) == "" {
			out = append(out, prefix)
		}
	}
	return out
}
