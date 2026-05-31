class LineEntity {
    constructor(data) {
        this.id = data.name;
        this.name = data.name;
        this.x = data.x || 0;
        this.y = data.y || 0;
        this.isOnline = data.isOnline;
        this.currentMaterial = null;
        this.currentCount = 0;
        this.targetCount = 0;        // Целевое значение для плавности
        this.maxCount = 100;
        this.printer = data.printer;
        this.ip = data.ip;
        this.pulseIntensity = 0;
        this.cellSize = data.cellSize || 100;
    }
    
    updateFromAPI(data) {
        this.isOnline = data.isOnline;
    }
    
    updateProduction(material, count, maxCount) {
        this.currentMaterial = material;
        this.targetCount = count;
        this.maxCount = maxCount || 100;
        this.pulseIntensity = 0.8;
    }
    
    update() {
        // Плавное обновление счётчика
        if (Math.abs(this.currentCount - this.targetCount) > 0.1) {
            this.currentCount += (this.targetCount - this.currentCount) * 0.2;
        } else {
            this.currentCount = this.targetCount;
        }
    }
    
    draw(ctx, camera) {
        // Обновляем анимацию
        this.update();
        
        const size = this.cellSize;
        const radius = size / 2;
        
        ctx.shadowBlur = 15;
        ctx.shadowColor = this.isOnline ? '#00ff0040' : '#ff000040';
        
        ctx.fillStyle = this.isOnline ? '#1a2a1a' : '#2a1a1a';
        ctx.fillRect(this.x - radius, this.y - radius, size, size);
        
        ctx.strokeStyle = this.isOnline ? '#0f0' : '#f00';
        ctx.lineWidth = 2;
        ctx.strokeRect(this.x - radius, this.y - radius, size, size);
        
        ctx.font = `bold ${14}px monospace`;
        ctx.fillStyle = '#ccc';
        ctx.textAlign = 'center';
        ctx.fillText(this.name, this.x, this.y - radius + 15);
        
        ctx.font = `10px monospace`;
        ctx.fillStyle = this.isOnline ? '#0f0' : '#f00';
        ctx.fillText(this.isOnline ? '● ONLINE' : '○ OFFLINE', this.x, this.y - radius + 28);
        
        const fillPercent = Math.min(1, this.currentCount / this.maxCount);
        const barWidth = size - 10;
        const barHeight = 4;
        const barX = this.x - radius + 5;
        const barY = this.y + radius - 10;
        
        ctx.fillStyle = '#2a2a2a';
        ctx.fillRect(barX, barY, barWidth, barHeight);
        ctx.fillStyle = '#ffaa00';
        ctx.fillRect(barX, barY, barWidth * fillPercent, barHeight);
        
        if (this.currentMaterial) {
            ctx.font = `10px monospace`;
            ctx.fillStyle = '#0f0';
            const displayCount = Math.round(this.currentCount);
            ctx.fillText(`${this.currentMaterial} ${displayCount}/${this.maxCount}`, 
                         this.x, this.y + radius - 5);
        }
        
        if (this.pulseIntensity > 0) {
            ctx.fillStyle = `rgba(0, 255, 0, ${this.pulseIntensity * 0.3})`;
            ctx.fillRect(this.x - radius, this.y - radius, size, size);
            this.pulseIntensity *= 0.9;
        }
        
        ctx.shadowBlur = 0;
    }
}