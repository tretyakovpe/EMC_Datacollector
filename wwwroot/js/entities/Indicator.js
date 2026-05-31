class IndicatorEntity {
    constructor(x, y, type, value) {
        this.x = x;
        this.y = y;
        this.type = type;   // 'good', 'bad', 'box'
        this.value = value;
        this.life = 1.0;    // умирает со временем
        this.velocityY = -1;
    }
    
    update() {
        this.y += this.velocityY;
        this.life -= 0.02;
        return this.life > 0;
    }
    
    draw(ctx, camera) {
        const screen = camera.worldToScreen(this.x, this.y);
        const alpha = Math.min(1, this.life * 2);
        
        ctx.globalAlpha = alpha;
        
        if (this.type === 'good') {
            ctx.fillStyle = '#0f0';
            ctx.font = `${16 * camera.zoom}px monospace`;
            ctx.fillText('✓', screen.x, screen.y);
        } else if (this.type === 'bad') {
            ctx.fillStyle = '#f00';
            ctx.font = `${16 * camera.zoom}px monospace`;
            ctx.fillText('✗', screen.x, screen.y);
        } else if (this.type === 'box') {
            ctx.fillStyle = '#ff0';
            ctx.font = `${20 * camera.zoom}px monospace`;
            ctx.fillText('📦', screen.x, screen.y);
        }
        
        ctx.globalAlpha = 1;
    }
}