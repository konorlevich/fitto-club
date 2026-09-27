package content

// Facts that change by deploy, not through the admin (BRIEF.md §4).

const (
	Phone        = "+995595031885"
	PhoneDisplay = "+995 595 03 18 85"
	Instagram    = "fitto.club_tbilisi"
	InstagramDM  = "https://ig.me/m/fitto.club_tbilisi"
	Lat          = 41.7106845
	Lon          = 44.7558247
	// Google's own share link for the place card: it opens the listing with
	// reviews and the route button, in the app when installed.
	GoogleMapsURL = "https://maps.app.goo.gl/3LpmtjEe7dwZARcEA"
	// Directions deep links. Apple Maps falls back to the web on non-Apple
	// devices, so both buttons always work.
	GoogleDirURL = "https://www.google.com/maps/dir/?api=1&destination=41.7106845%2C44.7558247"
	AppleDirURL  = "https://maps.apple.com/?daddr=41.7106845,44.7558247&q=Fitto%20Club"
	GoogleRating = "4.6"
	GoogleCount  = 48
	Founded      = "2025-07"
)

// Address is shown in all three scripts on every locale: a taxi driver reads
// Georgian, a visitor may not.
var Address = struct {
	Street L
	Area   L
}{
	Street: L{"en": "9 Ilo Mosashvili St", "ru": "ул. Ило Мосашвили, 9", "ka": "ილო მოსაშვილის ქ. 9"},
	Area:   L{"en": "Vake, Tbilisi", "ru": "Ваке, Тбилиси", "ka": "ვაკე, თბილისი"},
}

var Zones = []Zone{
	{
		Key: "hall", Photo: "hall", W: 4, H: 3,
		Name: L{"en": "Gym floor", "ru": "Тренажёрный зал", "ka": "დარბაზი"},
		Text: L{
			"en": "New machines and free weights under the club's pink and blue light columns.",
			"ru": "Новые тренажёры и свободные веса под розово-синими световыми колоннами.",
			"ka": "ახალი ტრენაჟორები და თავისუფალი წონები ვარდისფერ-ლურჯი სინათლის სვეტების ქვეშ.",
		},
		Alt: L{
			"en": "The gym floor with strength machines lit by pink and blue columns",
			"ru": "Тренажёрный зал с силовыми тренажёрами в розово-синей подсветке",
			"ka": "დარბაზი ძალოვანი ტრენაჟორებით, ვარდისფერ-ლურჯი განათებით",
		},
	},
	{
		Key: "crossfit", Photo: "crossfit", W: 4, H: 3,
		Name: L{"en": "CrossFit zone", "ru": "CrossFit-зона", "ka": "CrossFit-ზონა"},
		Text: L{
			"en": "A separate room with a red turf track, rings and kettlebells, so functional training never queues behind the bench.",
			"ru": "Отдельный зал с красной дорожкой, кольцами и гирями: функциональным тренировкам не приходится ждать, пока освободится скамья для жима.",
			"ka": "ცალკე ოთახი წითელი ბილიკით, რგოლებითა და გირებით, ასე რომ ფუნქციური ვარჯიშისთვის სკამთან რიგში დგომა არ მოგიწევთ.",
		},
		Alt: L{
			"en": "CrossFit room with a red turf track and gymnastic rings",
			"ru": "CrossFit-зона с красной дорожкой и гимнастическими кольцами",
			"ka": "CrossFit-ზონა წითელი ბილიკითა და ტანვარჯიშის რგოლებით",
		},
	},
	{
		Key: "cardio", Photo: "cardio", W: 4, H: 3,
		Name: L{"en": "Cardio", "ru": "Кардио", "ka": "კარდიო"},
		Text: L{
			"en": "Treadmills and bikes in their own zone, away from the weights.",
			"ru": "Беговые дорожки и велотренажёры в отдельной зоне, подальше от железа.",
			"ka": "სარბენი ბილიკები და ველოტრენაჟორები ცალკე ზონაში, წონებისგან მოშორებით.",
		},
		Alt: L{
			"en": "Cardio machines under the mezzanine, lit in purple",
			"ru": "Кардиотренажёры под антресолью в фиолетовой подсветке",
			"ka": "კარდიო ტრენაჟორები ანტრესოლის ქვეშ, იისფერი განათებით",
		},
	},
	{
		Key: "massage", Photo: "massage-wide", W: 4, H: 3,
		Name: L{"en": "Massage room", "ru": "Массажный кабинет", "ka": "მასაჟის ოთახი"},
		Text: L{
			"en": "A quiet room for recovery after training, or instead of it.",
			"ru": "Тихий кабинет для восстановления после тренировки — или вместо неё.",
			"ka": "მშვიდი ოთახი აღდგენისთვის ვარჯიშის შემდეგ — ან მის ნაცვლად.",
		},
		Alt: L{
			"en": "Massage room with a black table and warm lamp light",
			"ru": "Массажный кабинет: чёрный стол и тёплый свет лампы",
			"ka": "მასაჟის ოთახი შავი მასაჟის მაგიდითა და ნათურის თბილი შუქით",
		},
	},
	{
		Key: "lounge", Photo: "lounge-wide", W: 4, H: 3,
		Name: L{"en": "Lounge", "ru": "Лаунж", "ka": "ლაუნჯი"},
		Text: L{
			"en": "Sofas by the living plant wall, water and coffee at the desk.",
			"ru": "Диваны у стены из живых растений, вода и кофе на ресепшене.",
			"ka": "დივნები ცოცხალ მცენარეთა კედელთან, წყალი და ყავა რესეფშენზე.",
		},
		Alt: L{
			"en": "Lounge with sofas and the glowing Fitto Club sign",
			"ru": "Лаунж с диванами и светящейся вывеской Fitto Club",
			"ka": "ლაუნჯი დივნებითა და Fitto Club-ის მანათობელი აბრით",
		},
	},
	{
		Key: "lockers", Photo: "lockers", W: 4, H: 3,
		Name: L{"en": "Changing rooms", "ru": "Раздевалки", "ka": "გასახდელები"},
		Text: L{
			"en": "Lockers with code locks and showers. The lockers are small, so bring a compact bag.",
			"ru": "Шкафчики с кодовым замком и душевые. Шкафчики небольшие — лучше взять компактную сумку.",
			"ka": "კოდიანი კარადები და საშხაპეები. კარადები პატარაა, ამიტომ სჯობს, პატარა ჩანთა წამოიღოთ.",
		},
		Alt: L{
			"en": "Changing room with numbered white lockers and a bench",
			"ru": "Раздевалка с белыми нумерованными шкафчиками и скамейкой",
			"ka": "გასახდელი დანომრილი თეთრი კარადებითა და სკამით",
		},
	},
}

var Partners = []Partner{
	{Name: "UltraSave Clinic", Text: L{
		"en": "Sports check-ups with our coaches.",
		"ru": "Спортивный чек-ап вместе с нашими тренерами.",
		"ka": "სპორტული სამედიცინო შემოწმება ჩვენს მწვრთნელებთან ერთად.",
	}},
	{Name: "Poke Bar Tbilisi", Text: L{
		"en": "Food after training, a few minutes away.",
		"ru": "Поесть после тренировки — в паре минут от клуба.",
		"ka": "საჭმელი ვარჯიშის შემდეგ, კლუბიდან რამდენიმე წუთში.",
	}},
	{Name: "Corner Tbilisi", Text: L{
		"en": "The coffee shop in our building.",
		"ru": "Кофейня в нашем здании.",
		"ka": "ყავახანა ჩვენსავე შენობაში.",
	}},
	{Name: "JET", Text: L{
		"en": "Power bank station at the desk.",
		"ru": "Станция пауэрбанков на ресепшене.",
		"ka": "პაუერბანკების სადგური რესეფშენზე.",
	}},
	{Name: "Chognari Dogs", Text: L{
		"en": "Our donation box goes to this shelter.",
		"ru": "Всё из нашего ящика для пожертвований уходит в этот приют.",
		"ka": "კლუბის შემოწირულობების ყუთიდან თანხა ამ თავშესაფარს ხმარდება.",
	}},
}
