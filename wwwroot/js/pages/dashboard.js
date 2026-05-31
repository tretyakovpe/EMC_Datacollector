// Страница дашборда (логи + статистика)
class DashboardPage {
    constructor() {
        this.logContainer = null;
        this.updateInterval = null;
    }
    
    async render(container) {
        container.innerHTML = `
            <div class="row mb-4">
                <div class="col-md-3">
                    <div class="card text-center">
                        <div class="card-body">
                            <h3 id="totalGood">0</h3>
                            <p>✅ Годных деталей</p>
                        </div>
                    </div>
                </div>
                <div class="col-md-3">
                    <div class="card text-center">
                        <div class="card-body">
                            <h3 id="totalBad">0</h3>
                            <p>❌ Брак</p>
                        </div>
                    </div>
                </div>
                <div class="col-md-3">
                    <div class="card text-center">
                        <div class="card-body">
                            <h3 id="totalBoxes">0</h3>
                            <p>📦 Коробок</p>
                        </div>
                    </div>
                </div>
                <div class="col-md-3">
                    <div class="card text-center">
                        <div class="card-body">
                            <h3 id="onlineLines">0</h3>
                            <p>🔌 Линий онлайн</p>
                        </div>
                    </div>
                </div>
            </div>
            <div class="row">
                <div class="col-12">
                    <div class="card">
                        <div class="card-header d-flex justify-content-between">
                            <span>📝 Системные логи</span>
                            <button class="btn btn-sm btn-secondary" id="clearLogsBtn">Очистить</button>
                        </div>
                        <div class="card-body">
                            <div id="logContainer" style="background:#1e1e1e; color:#d4d4d4; height:400px; overflow-y:auto; padding:10px; font-family:monospace; font-size:12px;"></div>
                        </div>
                    </div>
                </div>
            </div>
        `;
        
        this.logContainer = document.getElementById('logContainer');
        
        // Кнопка очистки
        document.getElementById('clearLogsBtn')?.addEventListener('click', () => {
            if (this.logContainer) this.logContainer.innerHTML = '';
        });
        
        return this;
    }
    
    onActivate(ws, logHandler) {
        // Регистрируем обработчик логов
        window.App.setLogHandler((level, message, time) => {
            this.addLog(level, message, time);
        });
        
        // Загружаем статистику
        this.loadStats();
        this.updateInterval = setInterval(() => this.loadStats(), 5000);
    }
    
    addLog(level, message, time) {
        if (!this.logContainer) return;
        const logClass = level === 'ERROR' ? 'color:#f48771' : 'color:#4ec9b0';
        const div = document.createElement('div');
        div.style.cssText = logClass;
        div.innerHTML = `[${time}] [${level}] ${this.escapeHtml(message)}`;
        this.logContainer.appendChild(div);
        this.logContainer.scrollTop = this.logContainer.scrollHeight;
        
        // Ограничиваем количество
        while (this.logContainer.children.length > 500) {
            this.logContainer.removeChild(this.logContainer.firstChild);
        }
    }
    
    async loadStats() {
        try {
            const stats = await API.getStats();
            document.getElementById('totalGood').textContent = Object.values(stats.goodParts || {}).reduce((a,b) => a+b, 0);
            document.getElementById('totalBad').textContent = Object.values(stats.badParts || {}).reduce((a,b) => a+b, 0);
            document.getElementById('totalBoxes').textContent = stats.boxesClosed || 0;
            
            const lines = await API.getLines();
            const online = lines.filter(l => l.isOnline).length;
            document.getElementById('onlineLines').textContent = online;
        } catch (error) {
            console.error('Stats error:', error);
        }
    }
    
    escapeHtml(text) {
        const div = document.createElement('div');
        div.textContent = text;
        return div.innerHTML;
    }
    
    destroy() {
        if (this.updateInterval) clearInterval(this.updateInterval);
    }
}