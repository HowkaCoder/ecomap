package telegram

import (
	"context"
	"strings"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

var karakalpakMessages = map[string]string{
	"Не понял команду. Откройте /help.": "Komanda túsiniksiz. Kómek ushın /help jiberiń.",
	"🌱 <b>EcoMap</b>\n\n♻️ Контейнеры — /bins\n🚚 График — /calendar\n⚠️ Пропущенный вывоз — /missed\n🗑 Сообщить о свалке — кнопка в меню\n🧭 Сортировка — /sort\n📋 Мои обращения — /myreports\n🌿 Подтверждённый вес — /impact\n🔔 Напоминания — /reminders\n🆔 Мой ID — /id\n\nСборщикам: /batch, затем сканируйте QR контейнера камерой телефона и введите вес в боте. Завершение рейса: /finishbatch.": "🌱 <b>EcoMap kómek</b>\n\n♻️ Konteynerler — /bins\n🚚 Keste — /calendar\n⚠️ Alıp ketiw qaldı — /missed\n🗑 Qoqıs haqqında xabar — menyuda\n🧭 Sortlaw — /sort\n📋 Meniń múrájatlarım — /myreports\n🌿 Tastıyıqlanǵan salmaq — /impact\n🔔 Esletpeler — /reminders\n🗣 Til — /language\n🆔 Telegram ID — /id\n\nJıynawshı: /batch → konteyner QR-in skanerleń → salmaǵın kiritiń. Reysti juwmaqlaw: /finishbatch.",
	"Нажмите /start, чтобы открыть меню.": "/start jiberip, menyudi ashıń.",
	"📸 Отправьте фотографию свалки.": "📸 Qoqıstıń súwretin jiberiń.",
	"📍 Отправьте геолокацию свалки.": "📍 Qoqıstıń jaylasqan ornın jiberiń.",
	"📍 Нужна геолокация. Нажмите скрепку → Геопозиция.": "📍 Geolokaciya kerek. Qısqısh → Geopoziciya bólimin tańlań.",
	"🏠 Укажите адрес, если знаете его.": "🏠 Mánzildi bilseńiz, jazıń.",
	"🧭 Добавьте ориентир: школу, магазин или перекрёсток.": "🧭 Baǵdar jazıń: mektep, dúkan yamasa kóshe kesilisi.",
	"📝 Кратко опишите проблему.": "📝 Máseleni qısqasha túsindiriń.",
	"🚨 Укажите срочность: обычная, опасная или срочно.": "🚨 Jеделлиgin kórsetiń: ápiwayı, qáwipli yamasa tez.",
	"Не удалось сохранить обращение. Попробуйте ещё раз.": "Múrájat saqlanbadı. Qayta urınıp kóriń.",
	"Введите имя текстом.": "Atıńızdı jazıń.",
	"📸 Отправьте фото одного из контейнеров.": "📸 Konteynerlerden biriniń súwretin jiberiń.",
	"📸 Нужна фотография контейнера.": "📸 Konteyner súwreti kerek.",
	"🏠 Укажите адрес дома или точки сбора.": "🏠 Úydiń yamasa jıynaw ornınıń mánzilin jazıń.",
	"📍 Отправьте геолокацию точки сбора.": "📍 Jıynaw ornınıń geolokaciyasın jiberiń.",
	"Введите вес числом в килограммах, например 8.5.": "Salmaqtı kilogramm menen kiritiń, mısalı 8.5.",
	"Вес должен быть больше 0 и не более 1 000 кг.": "Salmaq 0-den úlken hám 1 000 kg-dan aspawı kerek.",
	"Этот QR относится к другой фракции. Создайте рейс нужного типа через /batch.": "Bul QR basqa materialǵa tiyisli. /batch arqalı sáykes reys ashıń.",
	"QR-код контейнера не найден.": "Konteyner QR-kodi tabılmadı.",
	"Не удалось создать обращение. Проверьте контейнер и попробуйте снова.": "Múrájat jaratılmadı. Konteynerdi tekserip, qayta urınıń.",
	"Выберите действие в меню.": "Menyuden ámeldi tańlań.",
	"Давайте подключим дом к раздельному сбору. Как вас зовут?": "Úyińizdi bólek jıynawǵa qosamız. Atıńız kim?",
	"Не удалось загрузить контейнеры.": "Konteynerler júklenbedi.",
	"Контейнеров пока нет.": "Házir konteyner joq.",
	"🚚 График ещё не настроен. Оператор задаёт расписание через переменную PICKUP_SCHEDULE в формате mon:plastic,tue:paper,fri:glass.": "🚚 Alıp ketiw kestesi ele sazlanbaǵan. Operator PICKUP_SCHEDULE arqalı kesteni belgilewi kerek.",
	"🧭 <b>Что куда?</b>\n\n🧴 <b>Пластик</b> — пустая чистая упаковка; снимите остатки еды.\n📄 <b>Бумага</b> — сухая бумага и картон, без жирных и мокрых загрязнений.\n🍾 <b>Стекло</b> — бутылки и банки; керамику и лампы не кладите.\n\nТочный список принимает оператор и местная площадка переработки.": "🧭 <b>Qaysı material qay jerge?</b>\n\n🧴 <b>Plastik</b> — bos hám taza qaptama; azıq qaldıǵın shayıń.\n📄 <b>Qaǵaz</b> — qurǵaq qaǵaz hám karton; maylı yamasa ıǵal qaǵaz bolmasın.\n🍾 <b>Shiyshe</b> — banka hám shóreke; keramika hám lampalar emes.\n\nQabıl etiletuǵın anıq tizimdi operator hám qayta islew ornı belgileydi.",
	"Не удалось загрузить подтверждённые данные.": "Tastıyıqlanǵan maǵlıwmat júklenbedi.",
	"🌱 Пока нет принятого переработчиком материала. После закрытия рейса появится подтверждённый вес.": "🌱 Qayta islew orayı ele material qabıl etpedi. Reys juwmaqlanıp, salmaq tastıyıqlanǵannan keyin kórinedi.",
	"Не удалось загрузить ваши контейнеры.": "Sizdiń konteynerlerińiz júklenbedi.",
	"У вас пока нет обращений.": "Házir sizde múrájat joq.",
	"Раздел доступен только сборщикам пилота.": "Bul bólim tek sınaw jıynawshıları ushın.",
	"Не удалось проверить активный рейс.": "Bel sendi reys tekserilmedi.",
	"Выберите фракцию для нового рейса:": "Jańa reys ushın materialdı tańlań:",
	"QR-код доступен сборщикам пилота. Если вы житель, откройте свои контейнеры через /bins.": "QR tek jıynawshılar ushın. Qala turǵını bolsańız, konteynerleri /bins arqalı kóriń.",
	"Не удалось проверить рейс.": "Reys tekserilmedi.",
	"Сначала откройте рейс командой /batch, выберите фракцию, затем повторно отсканируйте QR.": "Dáslep /batch arqalı reys ashıp, materialdı tańlań, soń QR-di qayta skanerleń.",
	"Команда доступна сборщикам тестового пилота.": "Bul komanda tek sınaw jıynawshıları ushın.",
	"Нет доступа.": "Ruxsat joq.",
	"Это не ваш контейнер.": "Bul sizdiń konteynerińiz emes.",
	"Не удалось обновить настройку.": "Sazlaw ózgertilmedi.",
	"🔔 Напоминания включены.": "🔔 Esletpeler qosıldı.",
	"🔕 Напоминания выключены.": "🔕 Esletpeler óshirildi.",
	"Не удалось завершить регистрацию. Попробуйте снова.": "Dizimnen ótiw juwmaqlanbadı. Qayta urınıń.",
	"Сначала подключите дом через «Регистрация».": "Dáslep «Úydi qosıw» bóliminde dizimnen ótiń.",
	"Не удалось загрузить заявки.": "Múrájatlar júklenbedi.",
	"Не удалось загрузить обращения.": "Xabarlar júklenbedi.",
	"Открытых заявок о пропуске вывоза нет.": "Alıp ketiw boyınsha ashıq múrájat joq.",
	"Открытых сообщений о свалках нет.": "Qoqıs haqqında ashıq xabar joq.",
	"Формат: /resolve_issue 123": "Formatı: /resolve_issue 123",
	"Формат: /resolve_dump 123": "Formatı: /resolve_dump 123",
	"🔔 Напоминания приходят вечером накануне вывоза.": "🔔 Esletpe alıp ketiwden bir kún aldın keshte keledi.",
	"Используйте /reminders on или /reminders off.": "/reminders on yamasa /reminders off qollanıń.",
}

func (h *Handler) loadLanguage(ctx context.Context, telegramID int64) {
	language, err := h.workflowUC.Language(ctx, telegramID)
	if err != nil || (language != "kaa" && language != "ru") {
		language = "kaa"
	}
	h.setLanguageCache(telegramID, language)
}

func (h *Handler) setLanguageCache(telegramID int64, language string) {
	h.mu.Lock()
	h.languages[telegramID] = language
	h.mu.Unlock()
}

func (h *Handler) language(telegramID int64) string {
	h.mu.Lock()
	defer h.mu.Unlock()
	if language := h.languages[telegramID]; language != "" {
		return language
	}
	return "kaa"
}

func (h *Handler) localize(chatID int64, message string) string {
	h.mu.Lock()
	language := h.languages[chatID]
	h.mu.Unlock()
	if language == "ru" {
		return message
	}
	if translated, ok := karakalpakMessages[message]; ok {
		return translated
	}
	return localizeCommonPhrases(message)
}

func (h *Handler) newMessage(chatID int64, message string) tgbotapi.MessageConfig {
	m := tgbotapi.NewMessage(chatID, h.localize(chatID, message))
	m.ParseMode = tgbotapi.ModeHTML
	return m
}

func localizeCommonPhrases(message string) string {
	replacements := [][2]string{
		{"Ваш Telegram ID:", "Sizdiń Telegram ID:"},
		{"Не удалось", "Ámelge aspadı"},
		{"Сначала", "Dáslep"},
		{"Выберите", "Tańlań"},
		{"выберите", "tańlań"},
		{"Команда доступна", "Komanda qoljetimli"},
		{"График вывоза", "Alıp ketiw kestesi"},
		{"график вывоза", "alıp ketiw kestesi"},
		{"Мои обращения", "Meniń múrájatlarım"},
		{"мои контейнеры", "meniń konteynerlerim"},
		{"Пластик", "Plastik"},
		{"Бумага", "Qaǵaz"},
		{"Стекло", "Shiyshe"},
		{"Смешанная фракция", "Aralas material"},
		{"Пустая", "Bos"},
		{"Нужна уборка", "Alıp ketiw kerek"},
		{"Открытых", "Ashıq"},
		{"включены", "qosıldı"},
		{"выключены", "óshirildi"},
		{"Не удалось загрузить", "Júklenbedi"},
		{"открыт для фракции", "material ushın ashıldı"},
		{"отправлен на приёмку", "qabıl etiwge jiberildi"},
		{"Завершить:", "Juwmaqlaw:"},
		{"Введите вес", "Salmaqtı kiritiń"},
		{"Рейс", "Reys"},
		{"открыт", "ashıldı"},
		{"Контейнер", "Konteyner"},
		{"контейнер", "konteyner"},
		{"Введите", "Kiritiń"},
		{"Вес", "Salmaq"},
		{"вес", "salmaq"},
		{"заявка", "múrájat"},
		{"обращение", "xabar"},
		{"закрыта", "jabıldı"},
		{"принято", "qabıl etildi"},
		{"собрано", "jıyıldı"},
		{"внесено", "qosıldı"},
		{"статус", "status"},
		{"Заявка", "Múrájat"},
		{"Обращение", "Xabar"},
		{"Понедельник", "Dúyshembi"},
		{"Вторник", "Siyshembi"},
		{"Среда", "Sárshembi"},
		{"Четверг", "Piyshembi"},
		{"Пятница", "Juma"},
		{"Суббота", "Shembi"},
		{"Воскресенье", "Ekshembi"},
		{"Подтверждённый", "Tastıyıqlanǵan"},
		{"Всего принято", "Jami qabıl etildi"},
		{"Ожидает приёмки", "Qabıl etiw kútilmekte"},
		{"пропущенный вывоз", "qaldırılǵan alıp ketiw"},
		{"Выберите контейнер", "Konteynerdi tańlań"},
		{"кг", "kg"},
		{"Завтра вывоз:", "Erteń alıp ketiw:"},
		{"Подготовьте соответствующие контейнеры.", "Kerekli konteynerlerdi tayarlap qoyıń."},
	}
	for _, replacement := range replacements {
		message = strings.ReplaceAll(message, replacement[0], replacement[1])
	}
	return message
}
