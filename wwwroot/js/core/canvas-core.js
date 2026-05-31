const CanvasCore = {
    canvas: null,
    ctx: null,
    width: 0,
    height: 0,

    init() {
        this.canvas = document.getElementById('gameCanvas');
        if (!this.canvas) {
            console.error('Canvas element not found!');
            return;
        }
        this.ctx = this.canvas.getContext('2d');
        this.resize();
        window.addEventListener('resize', () => this.resize());
        console.log('CanvasCore initialized');
    },

    resize() {
        this.width = window.innerWidth;
        this.height = window.innerHeight;
        this.canvas.width = this.width;
        this.canvas.height = this.height;
        Camera.updateViewport(this.width, this.height);
    },

    clear() {
        this.ctx.fillStyle = '#1a1a2e';
        this.ctx.fillRect(0, 0, this.width, this.height);
    },

    setFixedHeight(height) {
        this.height = Math.min(height, 800);
        this.canvas.height = this.height;
        this.canvas.style.height = this.height + 'px';
        Camera.updateViewport(this.width, this.height);
    }
};