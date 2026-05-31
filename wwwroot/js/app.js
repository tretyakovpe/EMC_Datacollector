let currentScreen = null;

function switchScreen(screenName) {
    if (currentScreen) {
        currentScreen.destroy();
    }
    
    const container = document.getElementById('screen-container');
    if (!container) return;
    
    switch (screenName) {
        case 'workshop':
            currentScreen = new WorkshopScreen();
            break;
        case 'warehouse':
            container.innerHTML = '<div class="text-center p-5">🚧 Склад ГП в разработке</div>';
            return;
        case 'shipping':
            container.innerHTML = '<div class="text-center p-5">🚧 Склад отгрузки в разработке</div>';
            return;
        default:
            currentScreen = new WorkshopScreen();
    }
    
    currentScreen.init();
}

// Инициализация
window.addEventListener('DOMContentLoaded', () => {
    // Табы
    document.querySelectorAll('[data-screen]').forEach(tab => {
        tab.addEventListener('click', () => {
            document.querySelectorAll('[data-screen]').forEach(t => t.classList.remove('active'));
            tab.classList.add('active');
            switchScreen(tab.dataset.screen);
        });
    });
    
    // Стартуем с цеха
    switchScreen('workshop');
});