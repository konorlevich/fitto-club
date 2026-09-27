> **Решение по проекту (27.09.2026):** панда используется только в логотипе, маскотом не делаем. Пункт «Mascot» в разделе «Differentiation opportunities» не применяется.

# Tbilisi gym websites: competitor visual audit for FITTO CLUB

**Method.** I searched with WebSearch and fetched each site's raw HTML and first-party CSS with curl. From that I counted the hex colours used, the `font-family` declarations, Google Fonts links, `hreflang` tags, button labels and headings. I also took a 1440px desktop screenshot of each homepage with headless Chrome. Hex values are the most-used colours in the CSS, so a few of them may be framework defaults; I note where that is likely. Date checked: 2026-09-27.

**Brands I could not find a site for.** Iron Gym, Fitness Palace, Crystal Fitness, Next Fitness, Pulse and Garage: none of the obvious `.ge` domains resolved and I found no search result for a Tbilisi gym under these names, so treat them as unverified or nonexistent. The Biltmore and Sheraton Metechi have hotel spa/gym pages only, with no standalone club site. Aspria's `aspriafitness.ge` does not resolve; `aspria.ge` is the live site.

---

## Sites reviewed (15)

### 1. Oktopus Fitness Club: https://oktopus.ge/
- **Area:** Vake (20 Nino Ramishvili St), plus City Mall Saburtalo and Lisi (pool and spa).
- **Palette:** Mostly black `#000` and white `#fff`, with grey text. There is no strong brand accent. The `#ff6900` in the CSS is WordPress's default preset, not a brand colour.
- **Type:** TBC Contractica and TBC Contractica CAPS, a Georgian corporate typeface, in heavy uppercase Georgian headings.
- **Hero:** A full-bleed dark, low-key photo or video of a muscular body with a big white uppercase headline, "გახდი ჩვენიანი" ("Become one of us"), and a small outlined button. A newsletter pop-up appears on load, plus a cookie banner.
- **Clichés:** Dark moody hero, Swiper sliders, three service cards (group training, studio pilates, personal training), a spa section and a mobile-app promo.
- **Languages:** Georgian by default, English through WPML.
- **Prices, schedule, trainers:** On subpages; the home page has only "learn more" links.
- **Main CTA:** "შემოგვიერთდი" (Join us).
- **Built with:** WordPress and Elementor.
- **FITTO should avoid:** A black, cinematic, sweaty-torso hero with white all-caps text. This is the nearest large competitor in Vake.

### 2. Snap Fitness Vake: https://www.snapfitness.com/ge/gyms/vake
- **Area:** Vake (29 Chavchavadze Ave).
- **Palette:** Corporate red `#c4161c` and `#b11419`, white, charcoal `#242526`, light grey `#f7f7f7`.
- **Type:** Poppins.
- **Hero:** A red diagonal panel over a stock photo of smiling members, with the script slogan "you've got this!".
- **Clichés:** Diagonal red bands, a "Money Back Guarantee" block, a video thumbnail, a membership band and a staffed-hours table.
- **Languages:** Georgian (`lang="ka-GE"`) and English.
- **Prices:** Not shown; it links out to memberships.
- **Main CTA:** "Book a Tour" and "Contact Us".
- **Built with:** Next.js, from the global franchise template.
- **FITTO should avoid:** A saturated red/orange-red band as the brand colour, and franchise-style stock photos of smiling members.

### 3. Aspria Fitness: https://www.aspria.ge/
- **Area:** Vake, Saburtalo, Dinamo/Didube, Dadiani, plus Batumi. Branches are open 24/7.
- **Palette:** White UI with a near-black photo hero. The accent is neon green `#21fe00` (the "Accept" button and logo), and the CSS also contains Tailwind's default green, orange and teal.
- **Type:** Inter and Plus Jakarta Sans, with BPG Arial Caps for Georgian.
- **Hero:** A full-bleed dark photo of a bodybuilder behind a barbell, with a centred "Aspria / Know why you are training" in plain sans.
- **Clichés:** Quote block, location cards, a "Meet Our Team" grid of 13 trainers by first name, service/membership cards with prices, gallery, FAQ accordion, and a dark-mode toggle.
- **Languages:** English and Georgian.
- **Main CTA:** Weak; mostly navigation (Locations, Services).
- **FITTO should avoid:** A neon-accent-on-black look. Aspria owns neon green; FITTO owns neon orange, so FITTO should not add a second neon colour.

### 4. Neptune Sports Complex (Silk Hospitality): https://silkhospitality.com/neptune-sports-complex/
- **Area:** Vake (49a Chavchavadze Ave). `neptune.ge` redirects here.
- **Palette:** Luxury-hotel neutrals: off-white `#f8f6f3` and `#f5f1eb`, taupe `#b6aa99`, charcoal `#1d1d1d`, deep green `#235540`.
- **Type:** Custom brand variables plus DM Sans.
- **Hero:** A full-screen photo slider (aqua-aerobics dumbbells, pool) with dots and arrows.
- **Clichés:** Hotel-group mega-nav, and "Get Your Membership" repeated five times.
- **Languages:** English and Georgian.
- **Main CTA:** "Get Your Membership".
- **FITTO should avoid:** Little overlap. This is the calm, beige, spa-luxury template.

### 5. Radisson Blu Iveria Fitness & Wellness (Silk Hospitality): https://silkhospitality.com/radisson-blu-tbilisi/fitness-wellness/
- **Area:** Rose Revolution Square, central Tbilisi.
- **Look:** Same template and palette as Neptune: beige, charcoal, DM Sans, photo slider.
- **Main CTA:** "Treatments & Memberships" and "Contact us".
- **Takeaway:** Hotel wellness in Tbilisi reads as quiet beige luxury.

### 6. Pullman Axis Towers Wellness: https://pullman.accor.com/en/hotels/tbilisi/A1F1/wellness.html (membership shop at https://shop.axtw.ge/)
- **Area:** Vake/Vera border (37M Chavchavadze Ave, 6th floor).
- **Palette:** Accor's dark green-black with mint accent buttons.
- **Hero:** A large wide display headline, "BEYOND WELL-BEING".
- **Membership shop:** `shop.axtw.ge` is a black-and-white e-commerce storefront (Inter/Barlow) selling memberships as products with "Shop now" buttons.
- **Languages:** English (the Accor site is multilingual).
- **FITTO should avoid:** Minimal black-and-white e-shop styling for memberships.

### 7. Prime Fit: https://primefit.ge/ (branches at /vazha etc.)
- **Area:** Saburtalo/Vake (13 M. Tamarashvili St) and Saburtalo (71 Vazha-Pshavela Ave).
- **Palette:** Deep navy `#00092d` and `#101038`, royal blue `#1b3fd9`, white.
- **Type:** Nunito Sans.
- **Hero:** The home page is a branch picker: two large rounded photo cards of swimming pools, each with the address in huge light caps and EN/GE pills.
- **Clichés:** Pool-and-spa focus, with a cookie toast.
- **Languages:** English and Georgian.
- **Built with:** Tilda.
- **Main CTA:** Choose a branch.
- **FITTO should avoid:** No colour conflict. Note that the "address as headline" pattern is already used here.

### 8. Vake Swimming Pool & Fitness Club: https://vsp.ge/
- **Area:** Vake (49b Chavchavadze Ave). Founded 1965.
- **Palette:** Light sky-blue pattern background (swim icons), white, blue buttons, grey text.
- **Type:** Roboto and Roboto Slab, with thin, light-weight Georgian display text that is hard to read.
- **Hero:** A left text block, "since 1965", with three photo tiles (pool / fitness / group classes).
- **Clichés:** Photo-mosaic café section and count-up widgets.
- **Languages:** Georgian, English.
- **Prices:** On a "Services & prices" page. Third-party listings quote a 90 GEL day pass and a 440 GEL unlimited monthly pass; I did not check these on the site itself.
- **Built with:** WordPress and WooCommerce.
- **Takeaway:** A family and heritage feel, far from FITTO.

### 9. Gymnasia: https://gymnasia.ge/ (Vake studio page: https://gymnasia.ge/c/gymnasia-vake)
- **Area:** Main site at Amashukeli St 14a (Lisi side); the Vake studio is at 11a Zakaria Paliashvili St.
- **Palette:** Dark photo/video hero with a green tint. Accent green `#12b24b`/`#4a8c25`, slate `#0f172a`/`#111827`, some pink `#ef539e`.
- **Type:** Outfit for headings, Inter for body.
- **Hero:** A full-bleed dark video with a pill badge ("Biggest Combat Gym in Tbilisi"), a large "Welcome to GYMNASIA" and green/ghost buttons.
- **Clichés:** Count-up stat chips (5000+ members, 50+ classes, 20+ coaches), a class-card grid, a membership builder (package, then term, then pay per visit), testimonials, an FAQ/chat widget and a floating WhatsApp button.
- **Prices:** Shown openly, for example 150 GEL a month gym, and class credits from 5 per month at 150 ₾ to 20 per month at 480 ₾.
- **Languages:** English and Georgian.
- **Built with:** Next.js.
- **Main CTA:** "BOOK CLASS".
- **FITTO should avoid:** Glassy stat chips on a dark video, and the generic modern "dark SaaS" gym look.

### 10. Zenith Fitness & Wellness Boutique: https://zenithfitness.ge/
- **Area:** Tbilisi; I could not verify the exact address. It is the best-known "boutique" competitor.
- **Palette:** Warm: amber/yellow `#fec112`, dark brown `#352b22`/`#181512`, warm off-white `#f8f7f6`, black.
- **Type:** Literata (serif) with Nunito.
- **Hero:** A full-bleed photo of a woman training, "PREMIUM FITNESS & MASSAGE STUDIO", and a lead form ("Get 50% discount and fitness testing for free") with a yellow submit button.
- **Clichés:** Lead-capture form in the hero, service tiles (personal / group / osteopathy / massage / rehab), a pricing section and team.
- **Languages:** English, Russian, Georgian.
- **Built with:** Tilda.
- **Main CTA:** "BOOK ONLINE".
- **FITTO should avoid:** A discount lead form in the hero, and the "recovery/wellness" tone.

### 11. Cage Training Club (Iviko's Cage): https://cage.ge/
- **Area:** 2 University St, Saburtalo/Vake edge.
- **Palette:** Black and near-black `#111`/`#151515`/`#1c1c1c`. Red accent `#ff3b3b`/`#ff3333`, white, Apple-style greys `#f5f5f7`/`#a1a1a6`, green `#34c759` for status, yellow on the cookie button.
- **Type:** Bebas Neue for display, Work Sans for body, and Arial Black is also declared.
- **Hero:** A split layout: black left panel with the condensed red all-caps headline "CROSS THE LINE" and white/red buttons, and a magenta-lit autoplay video on the right with a "Sound on" toggle.
- **Clichés:** Program cards, a "Meet Your Coaches" carousel, a community section, gear shop, a plan picker with in-site checkout and accounts, and an event promo ("CAGE NIGHT").
- **Prices:** Full plan picker on the page.
- **Languages:** English, Georgian, Russian.
- **Main CTA:** "Start Training" and "Book Free Class" ("No card required").
- **FITTO should avoid (highest risk):** This is the closest look to FITTO's brand: black, a hot red/orange-red accent, heavy condensed caps, and Arial Black in the stack. FITTO must not use black with a hot accent on condensed all-caps display type in a split hero.

### 12. TNT Functional Fitness: https://tnt-fit.club/
- **Area:** Saburtalo (Merab Aleksidze St 67/85). CrossFit box.
- **Palette:** Near-black `#191919`, pure red `#e60101`, white, grey.
- **Type:** Anton for display, Roboto and Poppins for body.
- **Hero:** A full-bleed darkened photo with a centred all-caps headline ("TRANSFORM YOUR FITNESS JOURNEY WITH TNT…" with the brand name in red) and a solid red button.
- **Clichés:** "REAL PEOPLE. REAL RESULTS." photo carousel, a bullet list of benefits, and "YOUR FIRST VISIT IS ON US!" with a form.
- **Languages:** English, Russian.
- **Built with:** Weblium.
- **Main CTA:** "Sign up for a training session" / "CLAIM YOUR FIRST SESSION".
- **FITTO should avoid:** Again black, a hot accent and heavy condensed caps (Anton). FITTO's orange on black would read as the same template.

### 13. Champions Academy / CrossFit 148: https://champ.ge/en
- **Area:** 148g Agmashenebeli Ave (Didube/Chugureti, not Vake). Premium, around 250–300+ GEL a month according to third-party guides.
- **Palette:** Warm near-black `#231f20`, white, greyscale photos; Material defaults (`#3f51b5`, `#f44336`) are in the CSS, and the cookie modal is periwinkle.
- **Type:** Roboto and Neue Helvetica Georgian.
- **Hero:** A black-and-white action photo banner, "Win The Battle Within", and a circular outlined "JOIN NOW" button. The logo is a circular crest.
- **Languages:** English, Georgian.
- **Built with:** An Angular single-page app; the home page is sparse when rendered.
- **Main CTA:** "GET STARTED" and "JOIN NOW".
- **FITTO should avoid:** A circular crest badge with a mascot-style head. Champions' logo is a round emblem around a head, so FITTO's panda should not sit inside a circular seal.

### 14. Underground Fitness & Boxing: https://underground.com.ge/en/index.html
- **Area:** Nutsubidze Plato (Saburtalo outskirts).
- **Palette:** Dark header, orange `#f57708` active-nav/"Join Us!" accents, white body, with red/green/magenta plan titles.
- **Type:** Raleway and Arial.
- **Hero:** A blurred gym photo with a centred image slider.
- **Pricing and trainers:** Three pricing columns (20₾ a day / 70₾ for 14 days / 100₾ a month) and a trainer grid of ID-style headshots.
- **Languages:** Georgian, English.
- **Built with:** Wix.
- **Takeaway:** Budget and DIY. It shows that orange on dark already appears among local gyms, though weakly.

### 15. Fitpass (aggregator): https://fitpass.ge/
- **Scope:** City-wide access pass covering 100+ venues, for example Vortex Fitness and Reform Sport Club.
- **Palette:** White with orange-red `#e6441f`, black, Bootstrap greys. The logo is black "FITPASS" with orange bars.
- **Type:** System and Bootstrap fonts.
- **Hero:** A gym photo with phone mockups on orange shapes, and "Use 60+ activities in 300+ venues with the Fitpass app".
- **Clichés:** Venue-card carousel with rating badges, city tabs, and app-store badges.
- **Language:** Georgian.
- **FITTO should avoid:** Fitpass orange `#e6441f` is close to FITTO's `#FF4401`. If FITTO is listed on Fitpass, FITTO's orange UI next to Fitpass's orange UI could blur the two, so FITTO needs a distinct secondary colour and graphic language.

---

## Common patterns among Tbilisi gym sites
- **Dark photo heroes:** A full-bleed dark gym photo or autoplay video with a muscular body and a centred or left-aligned white all-caps headline (Oktopus, Aspria, Gymnasia, TNT, Cage, Champions, Zenith).
- **Black with a hot accent:** The combat and CrossFit segment uses black or near-black with a hot red accent (Cage `#ff3b3b`, TNT `#e60101`, Snap `#c4161c`). Orange appears at Fitpass (`#e6441f`) and Underground (`#f57708`), and Aspria and Gymnasia use neon green.
- **Condensed display type:** Bebas Neue, Anton and heavy uppercase TBC Contractica, with generic sans for body (Inter, Poppins, Roboto, Nunito, Work Sans).
- **Generic CTAs:** "Join now / Get started / Start training / Book free class / First visit free". Lead forms and discount offers sit in the hero (Zenith, TNT), and pop-ups appear on load (Oktopus).
- **Standard sections:** Service or class card grid, a coach grid or carousel (usually first names on stock-style headshots), a pricing-card row or plan picker, FAQ accordion, count-up stats (Gymnasia; the counter widgets on Oktopus and VSP could not be checked visually), and a floating WhatsApp or chat button.
- **Site builders:** Mostly WordPress/Elementor, Tilda, Wix or Weblium, which produces a lot of template sameness.
- **Languages:** Nearly always Georgian and English, often Russian. The language switcher is usually a small flag or text toggle in the header.
- **Prices:** Premium and hotel clubs hide prices behind "memberships" or contact. Combat, CrossFit and budget gyms list them openly.
- **Hotel and spa clubs:** Beige or cream luxury with serif or refined sans and photo sliders (Silk Hospitality, Pullman).

## Differentiation opportunities for FITTO
- **Palette risk.** The black-plus-hot-accent space is crowded (Cage, TNT, Snap, Fitpass orange). FITTO should keep its black and `#FF4401` but break the template:
  - Use the orange as large flat fields: whole orange sections, or an orange footer.
  - Pair it with a third colour no competitor uses, such as a warm off-white or paper colour for light sections, or the panda's white as a graphic device.
  - Do not add red, neon green or yellow.
- **Typography.** Lead with the varsity/collegiate outlined letterforms from the "FITTO" logo, for example outlined slab or collegiate display for headings with a clean grotesk for body. No competitor uses varsity or athletic-department type, while Bebas and Anton are everywhere. Use Arial Black only for the "CLUB" wordmark echo, not as the headline font (Cage already declares it).
- **Mascot.** Build a personality system around the panda: stickers, reactions, and illustrated empty and loading states. No Tbilisi competitor has a character. Champions uses a circular crest with a head, so keep the panda out of a round seal.
- **Hero.** Do not use a dark sweaty-torso photo or video with white caps. Options are a graphic or typographic hero (huge outlined FITTO letters, panda, flat orange) or real, well-lit, candid photos of the actual small room and people.
- **Boutique honesty.** Competitors show scale ("5000+ members", "300+ venues"). FITTO can do the opposite: a small club, named coaches with real bios, and a visible cap on members or class sizes. Avoid count-up stat chips entirely.
- **Transparent pricing.** Show pricing plainly on the home page, like a varsity scoreboard or menu board, since premium competitors hide theirs. Avoid the standard three-card "most popular" layout and the discount lead forms and pop-ups.
- **Schedule as a feature.** Most sites bury schedules. A clean weekly timetable, including in Georgian, would stand out.
- **Neighbourhood identity.** Lean into Vake: the address (Ilo Mosashvili 9), a map and nearby landmarks, as a local club rather than a network.
- **Languages.** Offer Georgian, English and Russian, with properly set Georgian type (Georgian characters need a real Georgian font, not a fallback). Several competitors' Georgian typography is weak (for example VSP's thin display text).
- **CTAs.** Pick one distinctive, on-brand call to action, such as "Try a session" or a panda-voiced line, instead of "JOIN NOW", "Get Started" or "Book Free Class". Avoid on-load pop-ups.

## Sources
- Search and aggregator pages: [expathub.ge best gyms](https://expathub.ge/best-gyms-in-tbilisi-georgia/), [gymintbilisi.com – Vake](https://gymintbilisi.com/gyms-in-vake/), [roampads guide](https://www.roampads.com/blog/best-gyms-in-tbilisi-fitness-guide), [inyourpocket – Vake Pool](https://www.inyourpocket.com/tbilisi/vake-swimming-pool-and-fitness-centre_113928v), [crossfit.com – CrossFit 148](https://www.crossfit.com/gym/27276/crossfit-148), [Pullman wellness](https://pullman.accor.com/en/hotels/tbilisi/A1F1/wellness.html), [Radisson Blu Iveria](https://silkhospitality.com/radisson-blu-tbilisi/fitness-wellness/)
- Sites analysed: [oktopus.ge](https://oktopus.ge/), [snapfitness.com/ge/gyms/vake](https://www.snapfitness.com/ge/gyms/vake), [aspria.ge](https://www.aspria.ge/), [Neptune](https://silkhospitality.com/neptune-sports-complex/), [shop.axtw.ge](https://shop.axtw.ge/), [primefit.ge](https://primefit.ge/), [vsp.ge](https://vsp.ge/), [gymnasia.ge](https://gymnasia.ge/), [zenithfitness.ge](https://zenithfitness.ge/), [cage.ge](https://cage.ge/), [tnt-fit.club](https://tnt-fit.club/), [champ.ge](https://champ.ge/en), [underground.com.ge](https://underground.com.ge/en/index.html), [fitpass.ge](https://fitpass.ge/)

The homepage screenshots are in ``research/competitor-shots/``. That folder is session scratch space, so copy anything worth keeping.
