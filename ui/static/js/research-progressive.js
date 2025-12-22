// ===== RESEARCH PROGRESSIVE MODULE =====
// Research-specific progressive enhancements

(function() {
    'use strict';

    // Initialize when DOM is ready
    document.addEventListener('DOMContentLoaded', function() {
        console.log('[Research Progressive] Initializing research enhancements...');
        initializeFieldInventory();
    });

    // ===== FIELD INVENTORY ENHANCEMENT =====
    function initializeFieldInventory() {
        const fieldItems = document.querySelectorAll('.field-item');

        fieldItems.forEach(item => {
            // Enhanced click behavior for field selection
            item.addEventListener('click', function() {
                showFieldDetails(this.dataset.fieldName, this.dataset.fieldType, this);
            });
        });
    }

    function showFieldDetails(fieldName, fieldType, element) {
        // Remove previous selections
        document.querySelectorAll('.field-item').forEach(item => {
            item.classList.remove('ring-2', 'ring-blue-500');
        });

        // Highlight selected field
        element.classList.add('ring-2', 'ring-blue-500');

        // Show details (progressive enhancement)
        const detailsPanel = document.getElementById('field-details-panel');
        if (detailsPanel) {
            detailsPanel.innerHTML = `
                <div class="p-4">
                    <h3 class="font-semibold text-lg">${fieldName}</h3>
                    <p class="text-sm text-gray-600 mb-2">Type: ${fieldType}</p>
                    <div class="text-xs text-gray-500">
                        Click to select this field for research.
                    </div>
                </div>
            `;
            detailsPanel.classList.remove('hidden');
        }

        console.log('Selected field:', fieldName, fieldType);
    }

})();