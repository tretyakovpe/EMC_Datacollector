class LineEntity {
    constructor(data) {
        this.id = data.name;
        this.name = data.name;
        this.x = data.x || 0;      // позиция в мире
        this.y = data.y || 0;
        this.isOnline = data.isOnline;
        this.status = data.isOnline ? 'online' : 'offline';
        this.currentMaterial = null;
        this.currentCount = 0;
        this.maxCount = 100;        // объём коробки
        this.printer = data.printer;
        this.ip = data.ip;
        
        // Анимация
        this.pulseIntensity = 0;
        this.lastUpdate = Date.now();
    }
    
    updateFromAPI(lineData) {
        this.isOnline = lineData.isOnline;
        this.status = lineData.isOnline ? 'online' : 'offline';
    }
    
    updateProduction(material, count, maxCount) {
        this.currentMaterial = material;
        this.currentCount = count;
        this.maxCount = maxCount || 100;
        this.pulseIntensity = 0.8; // вспышка при производстве
    }
    
    draw(ctx, camera) {
        const screen = camera.worldToScreen(this.x, this.y);
        const size = 60 * camera.zoom;
        
        // База (здание/станок)
        ctx.save();
        ctx.shadowBlur = 10 * camera.zoom;
        ctx.shadowColor = this.isOnline ? '#00ff0040' : '#ff000040';
        
        // Корпус
        ctx.fillStyle = this.isOnline ? '#2a2a3e' : '#3a2a2e';
        ctx.fillRect(screen.x - size/2, screen.y - size/2, size, size);
        
        // Рамка статуса
        ctx.strokeStyle = this.isOnline ? '#0f0' : '#f00';
        ctx.lineWidth = 3 * camera.zoom;
        ctx.strokeRect(screen.x - size/2, screen.y - size/2, size, size);
        
        // Индикатор заполнения (сбоку)
        const fillPercent = this.currentCount / this.maxCount;
        const barHeight = size * fillPercent;
        ctx.fillStyle = '#ffaa00';
        ctx.fillRect(screen.x + size/2 + 5, screen.y + size/2 - barHeight, 8, barHeight);
        
        // Имя линии
        ctx.font = `${12 * camera.zoom}px monospace`;
        ctx.fillStyle = '#ccc';
        ctx.textAlign = 'center';
        ctx.fillText(this.name, screen.x, screen.y + size/2 + 15);
        
        // Материал и количество
        if (this.currentMaterial) {
            ctx.font = `${10 * camera.zoom}px monospace`;
            ctx.fillStyle = '#0f0';
            ctx.fillText(`${this.currentMaterial} ${this.currentCount}/${this.maxCount}`, 
                         screen.x, screen.y - size/2 - 5);
        }
        
        // Эффект пульсации при производстве
        if (this.pulseIntensity > 0) {
            ctx.fillStyle = `rgba(0, 255, 0, ${this.pulseIntensity * 0.5})`;
            ctx.fillRect(screen.x - size/2, screen.y - size/2, size, size);
            this.pulseIntensity *= 0.95;
        }
        
        ctx.restore();
    }
}