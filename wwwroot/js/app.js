// Роутер для страниц
const pages = {
    dashboard: DashboardPage,
    lines: LinesPage,
    boxes: BoxesPage,
    stats: StatsPage
};

let currentPage = null;
let ws = null;
let globalLogHandler = null;

// Функция смены страницы
async function navigateTo(pageName) {
    if (!pages[pageName]) {
        console.error('Страница не найдена:', pageName);
        return;
    }
    
    // Уничтожаем текущую страницу
    if (currentPage && currentPage.destroy) {
        currentPage.destroy();
    }
    
    // Создаём новую
    currentPage = new pages[pageName]();
    
    // Рендерим
    const container = document.getElementById('page-container');
    container.innerHTML = '<div class="text-center p-5">Загрузка...</div>';
    
    try {
        await currentPage.render(container);
        currentPage.onActivate(ws, globalLogHandler);
    } catch (error) {
        container.innerHTML = `<div class="alert alert-danger">Ошибка загрузки: ${error.message}</div>`;
    }
}

// WebSocket для логов (единое соединение)
function initWebSocket() {
    const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
    ws = new WebSocket(`${protocol}//${window.location.host}/ws`);
    
    ws.onopen = () => {
        console.log('WebSocket connected');
        if (globalLogHandler) {
            globalLogHandler('INFO', 'WebSocket подключён', new Date().toLocaleTimeString());
        }
    };
    
    ws.onmessage = (event) => {
        const data = JSON.parse(event.data);
        if (data.type === 'log' && globalLogHandler) {
            globalLogHandler(data.data.level, data.data.message, data.data.time);
        }
    };
    
    ws.onclose = () => {
        if (globalLogHandler) {
            globalLogHandler('ERROR', 'WebSocket отключён. Переподключение через 3 сек...', new Date().toLocaleTimeString());
        }
        setTimeout(initWebSocket, 3000);
    };
}

// Регистрация обработчика логов от активной страницы
function setLogHandler(handler) {
    globalLogHandler = handler;
}

// Инициализация приложения
window.addEventListener('load', () => {
    initWebSocket();
    
    // Подписка на клики по навигации
    document.querySelectorAll('[data-page]').forEach(btn => {
        btn.addEventListener('click', (e) => {
            document.querySelectorAll('[data-page]').forEach(b => b.classList.remove('active'));
            btn.classList.add('active');
            navigateTo(btn.dataset.page);
        });
    });
    
    // Стартовая страница
    navigateTo('dashboard');
});

// Экспортируем для страниц
window.App = {
    setLogHandler,
    getWS: () => ws,
    api: API
};
