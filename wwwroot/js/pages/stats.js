class StatsPage {
    async render(container) {
        container.innerHTML = `
            <div class="card">
                <div class="card-header">
                    <h5>📊 Детальная статистика</h5>
                </div>
                <div class="card-body">
                    <div id="statsContent">Загрузка...</div>
                </div>
            </div>
        `;
        
        const stats = await API.getStats();
        const lines = await API.getLines();
        
        let html = '<h6>Годные детали по линиям:</h6><ul>';
        for (const [line, count] of Object.entries(stats.goodParts || {})) {
            html += `<li>${line}: ${count} шт.</li>`;
        }
        html += '</ul><h6>Брак по линиям:</h6><ul>';
        for (const [line, count] of Object.entries(stats.badParts || {})) {
            html += `<li>${line}: ${count} шт.</li>`;
        }
        html += '</ul>';
        html += `<p><strong>Всего коробок сегодня:</strong> ${stats.boxesClosed || 0}</p>`;
        html += `<p><strong>Линий в системе:</strong> ${lines.length}</p>`;
        
        document.getElementById('statsContent').innerHTML = html;
        return this;
    }
    
    onActivate() { }
    destroy() { }
}