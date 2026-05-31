const Overlay = {
    showTooltip(text, x, y) {
        const tooltip = document.getElementById('tooltip');
        if (!tooltip) return;
        tooltip.innerHTML = text;
        tooltip.style.left = x + 15 + 'px';
        tooltip.style.top = y + 15 + 'px';
        tooltip.classList.remove('hidden');
    },
    
    hideTooltip() {
        const tooltip = document.getElementById('tooltip');
        if (tooltip) tooltip.classList.add('hidden');
    },
    
    showNotification(message, type = 'info') {
        // Простое уведомление в консоль пока
        console.log(`[${type}] ${message}`);
    }
};