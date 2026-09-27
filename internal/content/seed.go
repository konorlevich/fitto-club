package content

// Seed data: the editable content as collected on 2026-09-27 from the club's
// Instagram and Google Maps (materials/CONTENT.md). It is imported into the
// store on first boot; after that the admin owns it.
//
// Facts still waiting for the club's confirmation are listed in Pending and
// block a production boot until confirmed (TASKS.md #26).

// Pending lists facts that are shown with the best available value but are
// not yet confirmed by the club.
var Pending = []string{
	"monthly Standard membership price: 180 GEL (price list) vs 160 GEL (April 2026 posts)",
	"closing time: Instagram says 23:00 on weekdays, Google Maps showed 22:00",
	"coach roster and consent to publish photos and names",
	"group class durations (assumed 60 min) and the current schedule",
	"legal entity for the footer and the privacy policy",
}

// BaselineContentDate is when the shipped, non-editable copy last materially
// changed. A constant on purpose: never build or deploy time (checklist §6).
const BaselineContentDate = "2026-09-27"

// SingleVisitPrice is the drop-in price, GEL.
const SingleVisitPrice = 20

var SeedMemberships = []Membership{
	{
		Slug: "standard",
		Name: L{"en": "Standard", "ru": "Стандартный", "ka": "სტანდარტული"},
		Desc: L{
			"en": "The whole club: gym floor, CrossFit zone, cardio, lockers and showers, every day we are open.",
			"ru": "Весь клуб: зал, CrossFit-зона, кардио, раздевалки и душевые — каждый день, когда мы открыты.",
			"ka": "მთელი კლუბი: დარბაზი, CrossFit-ზონა, კარდიო, გასახდელები და საშხაპეები — ყოველდღე, როცა ღია ვართ.",
		},
		Terms:  []Term{{1, 180, 0}, {3, 490, 1}, {6, 870, 2}, {12, 1550, 3}},
		GiftPT: true, Sort: 1, Updated: BaselineContentDate,
	},
	{
		Slug: "family",
		Name: L{"en": "Family", "ru": "Семейный", "ka": "საოჯახო"},
		Desc: L{
			"en": "The same membership for two or more people: partners, parents and children, siblings or friends. Price per person.",
			"ru": "Тот же абонемент для двоих и более: пара, родители и дети, братья и сёстры, друзья. Цена указана за одного человека.",
			"ka": "იგივე აბონემენტი ორი ან მეტი ადამიანისთვის: წყვილები, მშობლები და შვილები, და-ძმები ან მეგობრები. ფასი მითითებულია ერთ ადამიანზე.",
		},
		Terms:  []Term{{1, 150, 0}, {3, 410, 1}, {6, 670, 2}, {12, 1350, 3}},
		GiftPT: true, Sort: 2, Updated: BaselineContentDate,
	},
}

var SeedTags = []Tag{
	{"weight-loss", L{"en": "Weight loss", "ru": "Снижение веса", "ka": "წონის კლება"}, 1},
	{"muscle-gain", L{"en": "Muscle gain", "ru": "Набор массы", "ka": "კუნთის მასა"}, 2},
	{"functional", L{"en": "Functional training", "ru": "Функциональный тренинг", "ka": "ფუნქციური ვარჯიში"}, 3},
	{"strength", L{"en": "Strength", "ru": "Силовая подготовка", "ka": "ძალა"}, 4},
	{"rehab", L{"en": "Back & rehab", "ru": "Спина и реабилитация", "ka": "ზურგი და რეაბილიტაცია"}, 5},
	{"posture", L{"en": "Posture & core", "ru": "Осанка и кор", "ka": "ტანადობა და კორპუსი"}, 6},
	{"kettlebell", L{"en": "Kettlebells", "ru": "Гири", "ka": "გირები"}, 7},
	{"bjj", L{"en": "BJJ & grappling", "ru": "BJJ и грэпплинг", "ka": "BJJ და გრეპლინგი"}, 8},
	{"bodybuilding", L{"en": "Bodybuilding", "ru": "Бодибилдинг", "ka": "ბოდიბილდინგი"}, 9},
	{"powerlifting", L{"en": "Powerlifting", "ru": "Пауэрлифтинг", "ka": "ფაუერლიფტინგი"}, 10},
	{"nutrition", L{"en": "Nutrition plans", "ru": "Планы питания", "ka": "კვების გეგმები"}, 11},
	{"competition", L{"en": "Competition prep", "ru": "Подготовка к соревнованиям", "ka": "შეჯიბრებისთვის მომზადება"}, 12},
	{"team-sports", L{"en": "Team sports", "ru": "Игровые виды спорта", "ka": "გუნდური სპორტი"}, 13},
	{"kids", L{"en": "Kids", "ru": "Дети", "ka": "ბავშვები"}, 14},
	{"running", L{"en": "Running", "ru": "Бег", "ka": "სირბილი"}, 15},
}

var SeedCoaches = []Coach{
	{
		Slug: "otar-chkadua", Photo: true, Instagram: "otar25chkadua", Sort: 1, Published: true,
		Name:    L{"en": "Otar Chkadua", "ru": "Отар Чкадуа", "ka": "ოთარ ჭკადუა"},
		Tags:    []string{"functional", "kettlebell", "weight-loss", "muscle-gain", "rehab"},
		Speaks:  []string{"ka", "ru"},
		PTPrice: 120,
		Bio: L{
			"en": "Founder of Fitto Club, 11 years of coaching. Functional training, kettlebell juggling (two-time amateur world champion), weight loss, muscle gain and rehabilitation. Regional sambo champion and grappling medallist, with a university degree in sport.",
			"ru": "Основатель Fitto Club, 11 лет тренерского опыта. Функциональный тренинг, жонглирование гирями (двукратный чемпион мира среди любителей), снижение веса, набор массы и реабилитация. Чемпион региона по самбо, призёр соревнований по грэпплингу, высшее спортивное образование.",
			"ka": "Fitto Club-ის დამფუძნებელი, მწვრთნელად 11 წელია მუშაობს. ფუნქციური ვარჯიში, გირების ჟონგლირება (მოყვარულთა შორის ორგზის მსოფლიო ჩემპიონი), წონის კლება, კუნთის მასის მომატება და რეაბილიტაცია. სამბოში რეგიონალური ჩემპიონი და გრეპლინგში პრიზიორი, აქვს უმაღლესი სპორტული განათლება.",
		},
	},
	{
		Slug: "nikolai-starodubcev", Photo: true, Instagram: "nikolaibjjkbs", Sort: 2, Published: true,
		Name:    L{"en": "Nikolai Starodubcev", "ru": "Николай Стародубцев", "ka": "ნიკოლაი სტაროდუბცევი"},
		Tags:    []string{"bjj", "strength", "competition"},
		Speaks:  []string{"en", "ru"},
		PTPrice: 100,
		Bio: L{
			"en": "BJJ and grappling coach for more than 10 years, competing athlete for 15. Helps people get stronger and move well without overload, at their own pace. Prepares fighters for competition; online supervision is available.",
			"ru": "Больше 10 лет тренирует BJJ и грэпплинг, 15 лет выступает сам. Помогает стать сильнее и двигаться правильно — без перегрузок, в своём темпе. Готовит бойцов к соревнованиям, есть онлайн-сопровождение.",
			"ka": "10 წელზე მეტია ბრაზილიური ჯიუ-ჯიცუსა და გრეპლინგის მწვრთნელია, 15 წელია — მოქმედი სპორტსმენი. ეხმარება ადამიანებს, გაძლიერდნენ და სწორად იმოძრაონ გადატვირთვის გარეშე, საკუთარ ტემპში. ამზადებს მებრძოლებს შეჯიბრებებისთვის; შესაძლებელია ონლაინ სუპერვიზია.",
		},
	},
	{
		Slug: "vlad-zapolskikh", Photo: true, Instagram: "zapolskikh_psychofit98", Sort: 3, Published: true,
		Name:    L{"en": "Vlad Zapolskikh", "ru": "Влад Запольских", "ka": "ვლად ზაპოლსკიხი"},
		Tags:    []string{"muscle-gain", "weight-loss", "functional", "posture"},
		Speaks:  []string{"ru"},
		PTPrice: 90, PTFrom: true,
		Bio: L{
			"en": "Muscle gain, fat loss, functional conditioning, posture and core. A background in national-level track and field, studies in anatomy, biomechanics and sports nutrition, and a clinical psychology course that helps with motivation. Packages and online coaching available.",
			"ru": "Набор массы, жиросжигание, функциональная подготовка, осанка и кор. Лёгкая атлетика на национальном уровне, обучение анатомии, биомеханике и спортивной диетологии. Изучает клиническую психологию — это помогает в работе с мотивацией и психосоматикой. Есть пакеты и онлайн-ведение.",
			"ka": "კუნთის მასის მომატება, ცხიმის წვა, ფუნქციური მომზადება, ტანადობის კორექცია და კუნთოვანი კორპუსი. აქვს ეროვნული დონის მსუბუქი ათლეტიკის გამოცდილება, შეისწავლა ანატომია, ბიომექანიკა და სპორტული დიეტოლოგია, ხოლო კლინიკური ფსიქოლოგიის კურსი მოტივაციაზე მუშაობაში ეხმარება. შესაძლებელია პაკეტები და ონლაინ წაყვანა.",
		},
	},
	{
		Slug: "gocha-butbaia", Photo: true, Instagram: "goch2_9", Sort: 4, Published: true,
		Name:    L{"en": "Gocha Butbaia", "ru": "Гоча Бутбая", "ka": "გოჩა ბუთბაია"},
		Tags:    []string{"bodybuilding", "nutrition", "weight-loss", "muscle-gain"},
		Speaks:  []string{"ka", "en"},
		PTPrice: 85,
		Bio: L{
			"en": "Fitness and bodybuilding coach and nutritionist. Builds meal plans for weight loss and muscle gain. 4th at an international tournament, 7th at the Georgian Bodybuilding Cup, 2nd in a fitness challenge.",
			"ru": "Тренер по фитнесу и бодибилдингу, нутрициолог. Составляет планы питания для снижения веса и набора массы. 4-е место на международном турнире, 7-е — на Кубке Грузии по бодибилдингу, 2-е — в фитнес-челлендже.",
			"ka": "ფიტნესისა და ბოდიბილდინგის მწვრთნელი, ნუტრიციოლოგი. ადგენს კვების გეგმებს წონის კლებისა და კუნთის მასის მომატებისთვის. საერთაშორისო ტურნირზე მე-4 ადგილი, საქართველოს თასზე ბოდიბილდინგში — მე-7, ფიტნეს ჩელენჯში — მე-2.",
		},
	},
	{
		Slug: "anastasia-novikova", Photo: true, Instagram: "une_fauconnerie", Sort: 5, Published: true,
		Name:    L{"en": "Anastasia Novikova", "ru": "Анастасия Новикова", "ka": "ანასტასია ნოვიკოვა"},
		Tags:    []string{"strength", "kettlebell", "competition", "bjj"},
		Speaks:  []string{"en", "ru"},
		PTPrice: 80,
		Bio: L{
			"en": "General strength, prehab, strength and conditioning for contact sports, kettlebell technique. Degree in medical psychology, S&C programme at Barcelona Innovation Hub. BJJ purple belt and AJP medallist. Also the club's massage therapist.",
			"ru": "Общая сила, профилактика травм, силовая и функциональная подготовка к контактным видам спорта, техника работы с гирями. Диплом по медицинской психологии, программа S&C от Barcelona Innovation Hub. Фиолетовый пояс BJJ, призёр AJP. А ещё она массажист клуба.",
			"ka": "ზოგადი ძალა, პრეჰაბი, ძალისა და გამძლეობის მომზადება საკონტაქტო სპორტის სახეობებისთვის, გირის ტექნიკა. უნივერსიტეტის ხარისხი სამედიცინო ფსიქოლოგიაში, S&C პროგრამა Barcelona Innovation Hub-ში. BJJ-ის იისფერი ქამარი, AJP-ის პრიზიორი. ასევე კლუბის მასაჟისტია.",
		},
	},
	{
		Slug: "irakli-nikolaishvili", Photo: true, Instagram: "irakli.nikolaishvili", Sort: 6, Published: true,
		Name:    L{"en": "Irakli Nikolaishvili", "ru": "Иракли Николаишвили", "ka": "ირაკლი ნიკოლაიშვილი"},
		Tags:    []string{"muscle-gain", "weight-loss", "nutrition"},
		Speaks:  []string{"ka", "en", "ru"},
		PTPrice: 70,
		Bio: L{
			"en": "Fitness coach focused on muscle gain and weight loss. Trained in nutrition science and writes personal meal plans alongside the training.",
			"ru": "Фитнес-тренер: набор массы и снижение веса. Изучал нутрициологию и составляет персональные планы питания в дополнение к тренировкам.",
			"ka": "ფიტნეს მწვრთნელი: კუნთის მასის მომატება და წონის კლება. ნუტრიციოლოგი — ვარჯიშთან ერთად ადგენს პერსონალურ კვების გეგმებს.",
		},
	},
	{
		Slug: "dmitrii-poshin", Photo: true, Instagram: "dmitrifittbilisi", Sort: 7, Published: true,
		Name:    L{"en": "Dmitrii Poshin", "ru": "Дмитрий Пошин", "ka": "დმიტრი პოშინი"},
		Tags:    []string{"rehab", "posture", "functional"},
		Speaks:  []string{"en", "ru"},
		PTPrice: 70,
		Bio: L{
			"en": "Back rehabilitation, core strength, joint mobility and posture. Five years in fitness and a degree in sport. Uses therapeutic exercise and functional gymnastics, and writes programmes for beginners and people who sit all day.",
			"ru": "Реабилитация спины, укрепление кора, подвижность суставов и осанка. 5 лет в фитнесе, высшее спортивное образование. Работает методами ЛФК и функциональной гимнастики, пишет программы для новичков и для тех, кто весь день сидит.",
			"ka": "ზურგის რეაბილიტაცია, კუნთოვანი კორპუსის გამაგრება, სახსრების მობილურობა და ტანადობა. 5 წელი ფიტნესში, უმაღლესი სპორტული განათლება. იყენებს სამკურნალო ვარჯიშს (LFK) და ფუნქციურ ტანვარჯიშს; ადგენს პროგრამებს დამწყებთათვის და მჯდომარე ცხოვრების წესის მქონე ადამიანებისთვის.",
		},
	},
	{
		Slug: "grigol-janoashvili", Photo: true, Instagram: "grigol_janoashvili", Sort: 8, Published: true,
		Name:    L{"en": "Grigol Janoashvili", "ru": "Григол Джаноашвили", "ka": "გრიგოლ ჯანოაშვილი"},
		Tags:    []string{"bodybuilding", "powerlifting", "muscle-gain", "weight-loss", "nutrition"},
		Speaks:  []string{"ka", "en", "ru"},
		PTPrice: 50,
		Bio: L{
			"en": "More than 15 years in fitness. Bodybuilding, powerlifting (mainly bench press), muscle gain, weight loss, individual programmes and meal plans. Prepares athletes for competition and judges for the Georgian Bodybuilding and Fitness Federation.",
			"ru": "Больше 15 лет в фитнесе. Бодибилдинг, пауэрлифтинг (в основном жим лёжа), набор массы, снижение веса, индивидуальные программы и питание. Готовит спортсменов к соревнованиям, региональный судья Федерации бодибилдинга и фитнеса Грузии.",
			"ka": "ფიტნესში — 15 წელზე მეტი. ბოდიბილდინგი, ფაუერლიფტინგი (ძირითადად bench press), კუნთის მასის მომატება, წონის კლება, ინდივიდუალური პროგრამები და კვების გეგმები. ამზადებს სპორტსმენებს შეჯიბრებისთვის; საქართველოს ბოდიბილდინგისა და ფიტნესის ფედერაციის რეგიონალური მსაჯი.",
		},
	},
	{
		Slug: "ivan-alimamedov", Photo: true, Instagram: "iva_trainer13", Sort: 9, Published: true,
		Name:    L{"en": "Ivan Alimamedov", "ru": "Иван Алимамедов", "ka": "ივან ალიმამედოვი"},
		Tags:    []string{"weight-loss", "muscle-gain", "kids", "functional"},
		Speaks:  []string{"ka", "en", "ru"},
		PTPrice: 60,
		Bio: L{
			"en": "Seven years of experience and a degree in sport. Weight loss, muscle gain and general fitness; works with children, with tennis players on functional conditioning, and with people who sit a lot.",
			"ru": "7 лет опыта, высшее спортивное образование. Снижение веса, набор массы и общая физическая форма. Работает с детьми, ставит функциональную подготовку теннисистам и занимается с теми, у кого сидячая работа.",
			"ka": "7-წლიანი გამოცდილება, უმაღლესი სპორტული განათლება. წონის კლება, კუნთის მასის მომატება და ზოგადი ფიზიკური ფორმა. მუშაობს ბავშვებთან, ჩოგბურთელებთან ფუნქციურ მომზადებაზე და მჯდომარე ცხოვრების წესის მქონე ადამიანებთან.",
		},
	},
	{
		Slug: "veriko-kundukhashvili", Photo: true, Instagram: "verachka_1", Sort: 10, Published: true,
		Name:    L{"en": "Veriko Kundukhashvili", "ru": "Верико Кундухашвили", "ka": "ვერიკო კუნდუხაშვილი"},
		Tags:    []string{"weight-loss", "strength", "team-sports"},
		Speaks:  []string{"ka", "en", "ru"},
		PTPrice: 50,
		Bio: L{
			"en": "Plays for Georgia's women's national rugby team (15s and 7s) and has 12 years in fitness. Weight loss, body shaping, muscle tone, endurance, speed and explosive power — for beginners and for athletes. World Rugby Level 1 referee.",
			"ru": "Игрок женской сборной Грузии по регби (регби-15 и регби-7), 12 лет в фитнесе. Снижение веса, коррекция фигуры, тонус, выносливость, скорость и взрывная сила — и для новичков, и для спортсменов. Судья World Rugby первого уровня.",
			"ka": "საქართველოს ქალთა რაგბის ეროვნული ნაკრების მოთამაშე (რაგბი 15 და რაგბი 7), ფიტნესში — 12 წელი. წონის კლება, სხეულის ფორმირება, კუნთოვანი ტონუსი, გამძლეობა, სისწრაფე და ფეთქებადი ძალა — როგორც დამწყებთათვის, ისე სპორტსმენებისთვის. World Rugby-ის პირველი დონის მსაჯი.",
		},
	},
	{
		Slug: "joni-nadoyan", Photo: true, Instagram: "nadoian95", Sort: 11, Published: true,
		Name:    L{"en": "Joni Nadoyan", "ru": "Джони Надоян", "ka": "ჯონი ნადოიანი"},
		Tags:    []string{"weight-loss", "muscle-gain", "nutrition"},
		Speaks:  []string{"ka", "ru"},
		PTPrice: 50,
		Bio: L{
			"en": "Degree in sports science. Weight loss through food and training, body recomposition, and muscle gain on a calorie surplus.",
			"ru": "Диплом в области спортивных наук. Снижение веса через питание и тренировки, рекомпозиция тела, набор массы на профиците калорий.",
			"ka": "განათლება სპორტის მეცნიერებაში. წონის კლება კვებისა და ვარჯიშის საშუალებით, სხეულის რეკომპოზიცია და კუნთოვანი მასის მომატება კალორიული პროფიციტით.",
		},
	},
	{
		Slug: "evan-kostylev", Photo: true, Instagram: "evankostylev", Sort: 12, Published: true,
		Name:    L{"en": "Evan Kostylev", "ru": "Эван Костылев", "ka": "ევან კოსტილევი"},
		Tags:    []string{"team-sports", "strength", "running"},
		Speaks:  []string{"en", "ru", "es", "de"},
		PTPrice: 35, PTCurrency: "USD",
		Bio: L{
			"en": "Active professional footballer who has played in the USA, Germany, Armenia and Georgia. Fitness for health, preparation for team sports, weightlifting. Runs the Friday morning running club.",
			"ru": "Действующий профессиональный футболист: играл в США, Германии, Армении и Грузии. Фитнес для здоровья, подготовка к командным видам спорта, тяжёлая атлетика. По пятницам утром ведёт беговой клуб.",
			"ka": "მოქმედი პროფესიონალი ფეხბურთელი — უთამაშია აშშ-ში, გერმანიაში, სომხეთსა და საქართველოში. ფიტნესი ჯანმრთელობისთვის, გუნდური სპორტის სპორტსმენების მომზადება, ძალოსნობა. პარასკეობით დილით სარბენ კლუბს უძღვება.",
		},
	},
	{
		// Not in the current "Coaches" highlight; kept unpublished so her old
		// links 301 to the list instead of 404-ing (BRIEF.md §4).
		Slug: "lizi-gagnidze", Photo: true, Instagram: "lizigagnidzee", Sort: 13, Published: false,
		Name:   L{"en": "Lizi Gagnidze", "ru": "Лизи Гагнидзе", "ka": "ლიზი გაგნიძე"},
		Tags:   []string{"muscle-gain"},
		Speaks: []string{"ka", "en"},
		Bio: L{
			"en": "Muscle gain, shape, tone and stretching.",
			"ru": "Набор массы, коррекция фигуры, тонус, растяжка.",
			"ka": "კუნთის მასა, ფორმა, ტონუსი და გაწელვა.",
		},
	},
}

var SeedClasses = []Class{
	{
		Slug: "full-body", Sort: 1, Price: 110,
		Name: L{"en": "Full Body", "ru": "Full Body", "ka": "Full Body"},
		Desc: L{
			"en": "Strength and functional training for the whole body in a small group, adapted to your level. Twelve sessions a month.",
			"ru": "Силовая и функциональная тренировка на всё тело в малой группе, под ваш уровень. 12 тренировок в месяц.",
			"ka": "ძალოვანი და ფუნქციური ვარჯიში მთელი სხეულისთვის მცირე ჯგუფში, თქვენს დონეზე მორგებული. თვეში 12 ვარჯიში.",
		},
		PriceNote: L{"en": "per month, three times a week", "ru": "в месяц, три раза в неделю", "ka": "თვეში, კვირაში სამჯერ"},
		Langs:     []string{"en", "ru"},
		Slots: []Slot{
			{1, "10:00", 60}, {1, "19:00", 60}, {3, "10:00", 60}, {3, "19:00", 60}, {5, "10:00", 60}, {5, "19:00", 60},
		},
		Updated: BaselineContentDate,
	},
	{
		Slug: "kettlebell", Sort: 2, Price: 80, Coach: "anastasia-novikova",
		Name: L{"en": "Kettlebell group", "ru": "Группа с гирями", "ka": "გირების ჯგუფი"},
		Desc: L{
			"en": "Swings, snatches and presses with a kettlebell technique coach. Strength, core and interval cardio without running. Small group, few places.",
			"ru": "Махи, рывки и жимы с тренером по технике работы с гирями. Сила, кор и интервальное кардио без бега. Малая группа, мест немного.",
			"ka": "ქნევები, აგლეჯები და წნევები გირის ტექნიკის მწვრთნელთან ერთად. ძალა, კორპუსი და ინტერვალური კარდიო სირბილის გარეშე. მცირე ჯგუფი, ადგილების რაოდენობა შეზღუდულია.",
		},
		PriceNote: L{"en": "per month once a week; 150 GEL twice a week", "ru": "в месяц, раз в неделю; 150 GEL — два раза в неделю", "ka": "თვეში, კვირაში ერთხელ; 150 GEL — ორჯერ"},
		Langs:     []string{"en", "ru"},
		Slots:     []Slot{{3, "14:00", 60}, {7, "12:00", 60}},
		Updated:   BaselineContentDate,
	},
	{
		Slug: "just-run", Sort: 3, Price: 10, Coach: "evan-kostylev",
		Name: L{"en": "Just Run", "ru": "Just Run", "ka": "Just Run"},
		Desc: L{
			"en": "Running club for people who love running and people who don't yet. Start from the club.",
			"ru": "Беговой клуб для тех, кто любит бегать, и для тех, кто пока не полюбил. Старт — у клуба.",
			"ka": "სარბენი კლუბი მათთვის, ვისაც სირბილი უყვარს, და მათთვისაც, ვისაც ჯერ არა. სტარტი კლუბიდან.",
		},
		PriceNote: L{"en": "per run", "ru": "за пробежку", "ka": "ერთ სირბილზე"},
		Langs:     []string{"en", "ru"},
		Slots:     []Slot{{5, "08:00", 60}},
		Updated:   BaselineContentDate,
	},
}

var SeedMassage = []MassageService{
	{Name: L{"en": "Full body massage", "ru": "Массаж всего тела", "ka": "მთელი სხეულის მასაჟი"}, Minutes: 60, Price: 130, Sort: 1},
	{Name: L{"en": "Full body massage", "ru": "Массаж всего тела", "ka": "მთელი სხეულის მასაჟი"}, Minutes: 90, Price: 170, Sort: 2},
	{Name: L{"en": "Back massage", "ru": "Массаж спины", "ka": "ზურგის მასაჟი"}, Minutes: 40, Price: 100, Sort: 3},
	{Name: L{"en": "Leg massage", "ru": "Массаж ног", "ka": "ფეხების მასაჟი"}, Minutes: 40, Price: 100, Sort: 4},
	{Name: L{"en": "Anti-cellulite massage", "ru": "Антицеллюлитный массаж", "ka": "ანტიცელულიტური მასაჟი"}, Minutes: 40, Price: 120, PackCount: 5, PackPrice: 500, Sort: 5},
	{Name: L{"en": "Anti-cellulite massage", "ru": "Антицеллюлитный массаж", "ka": "ანტიცელულიტური მასაჟი"}, Minutes: 60, Price: 150, PackCount: 5, PackPrice: 600, Sort: 6},
	{Name: L{"en": "Face massage", "ru": "Массаж лица", "ka": "სახის მასაჟი"}, Minutes: 40, Price: 80, Sort: 7},
	{Name: L{"en": "Back and face", "ru": "Спина и лицо", "ka": "ზურგი და სახე"}, Minutes: 60, Price: 130, Sort: 8},
}

var SeedHours = Hours{
	Week: [7]DayHours{
		{"08:00", "23:00"}, {"08:00", "23:00"}, {"08:00", "23:00"}, {"08:00", "23:00"}, {"08:00", "23:00"},
		{"09:00", "22:00"}, {"09:00", "22:00"},
	},
	Updated: BaselineContentDate,
}
