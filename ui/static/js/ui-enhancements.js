// ===== UI ENHANCEMENTS MODULE =====
// Progressive enhancements for better UX - works without JavaScript

(function() {
    'use strict';

    // Initialize when DOM is ready
    document.addEventListener('DOMContentLoaded', function() {
        console.log('[UI Enhancements] Initializing progressive enhancements...');
        initializeModalEnhancements();
        initializeKeyboardShortcuts();
        initializeAccessibilityHelpers();
        initializeTooltipEnhancements();
        initializeHypothesisExpansion();
    });

    // ===== MODAL ENHANCEMENTS =====
    function initializeModalEnhancements() {
        // Enhanced modal backdrop clicks (progressive enhancement)
        document.addEventListener('click', function(e) {
            if (e.target.classList.contains('modal-backdrop')) {
                closeActiveModal();
            }
        });

        // Enhanced ESC key handling for modals
        document.addEventListener('keydown', function(e) {
            if (e.key === 'Escape') {
                closeActiveModal();
            }
        });
    }

    function closeActiveModal() {
        // Find and close any open modal
        const openModal = document.querySelector('.modal[style*="display: flex"], .modal[style*="display: block"]');
        if (openModal) {
            openModal.style.display = 'none';
        }
    }

    // ===== KEYBOARD SHORTCUTS =====
    function initializeKeyboardShortcuts() {
        document.addEventListener('keydown', function(e) {
            // Ctrl/Cmd + K: Focus search (if exists)
            if ((e.ctrlKey || e.metaKey) && e.key === 'k') {
                e.preventDefault();
                const searchInput = document.querySelector('input[type="search"], input[placeholder*="search" i]');
                if (searchInput) {
                    searchInput.focus();
                    searchInput.select();
                }
            }

            // Ctrl/Cmd + /: Show keyboard shortcuts help
            if ((e.ctrlKey || e.metaKey) && e.key === '/') {
                e.preventDefault();
                showKeyboardShortcutsHelp();
            }
        });
    }

    function showKeyboardShortcutsHelp() {
        // Simple tooltip for shortcuts - could be enhanced with a modal
        const help = document.createElement('div');
        help.className = 'fixed top-4 right-4 bg-gray-900 text-white p-4 rounded-lg shadow-lg z-50 max-w-xs';
        help.innerHTML = `
            <div class="font-semibold mb-2">Keyboard Shortcuts</div>
            <div class="text-sm space-y-1">
                <div><kbd class="bg-gray-700 px-1 rounded">Ctrl+K</kbd> Focus search</div>
                <div><kbd class="bg-gray-700 px-1 rounded">Ctrl+/</kbd> Show this help</div>
                <div><kbd class="bg-gray-700 px-1 rounded">Esc</kbd> Close modals</div>
            </div>
        `;

        document.body.appendChild(help);
        setTimeout(() => help.remove(), 3000);
    }

    // ===== ACCESSIBILITY HELPERS =====
    function initializeAccessibilityHelpers() {
        // Add focus indicators for keyboard navigation
        document.addEventListener('keydown', function(e) {
            if (e.key === 'Tab') {
                document.body.classList.add('keyboard-navigation');
            }
        });

        document.addEventListener('mousedown', function() {
            document.body.classList.remove('keyboard-navigation');
        });

        // Enhanced skip links for screen readers
        const skipLink = document.createElement('a');
        skipLink.href = '#main-content';
        skipLink.className = 'sr-only focus:not-sr-only focus:absolute focus:top-2 focus:left-2 bg-blue-600 text-white px-4 py-2 rounded z-50';
        skipLink.textContent = 'Skip to main content';
        document.body.insertBefore(skipLink, document.body.firstChild);
    }

    // ===== HYPOTHESIS EXPANSION =====
    function initializeHypothesisExpansion() {
        // Handle hypothesis card expansion/collapse
        window.toggleHypothesisExpansion = function(cardElement) {
            const expandedSection = cardElement.querySelector('.hypothesis-expanded');
            const arrow = cardElement.querySelector('.hypothesis-arrow');

            if (expandedSection && arrow) {
                const isExpanded = !expandedSection.classList.contains('hidden');

                if (isExpanded) {
                    // Collapse
                    expandedSection.classList.add('hidden');
                    arrow.classList.remove('rotate-180');
                } else {
                    // Expand
                    expandedSection.classList.remove('hidden');
                    arrow.classList.add('rotate-180');
                }
            }
        };
    }

    // ===== TOOLTIP ENHANCEMENTS =====
    function initializeTooltipEnhancements() {
        // Add tooltips to elements with title attributes
        document.addEventListener('mouseover', function(e) {
            const target = e.target;
            if (target.hasAttribute('title') && !target.hasAttribute('data-tooltip-enhanced')) {
                enhanceTooltip(target);
            }
        });
    }

    function enhanceTooltip(element) {
        const title = element.getAttribute('title');
        if (!title) return;

        element.setAttribute('data-tooltip-enhanced', 'true');
        element.setAttribute('aria-label', title);

        // Keep title for accessibility but enhance with better UX
        element.addEventListener('mouseenter', function() {
            showEnhancedTooltip(this, title);
        });

        element.addEventListener('mouseleave', function() {
            hideEnhancedTooltip();
        });
    }

    let tooltipElement = null;

    function showEnhancedTooltip(target, content) {
        hideEnhancedTooltip(); // Remove any existing

        tooltipElement = document.createElement('div');
        tooltipElement.className = 'fixed bg-gray-900 text-white text-sm px-2 py-1 rounded shadow-lg pointer-events-none z-50 max-w-xs';
        tooltipElement.textContent = content;
        tooltipElement.setAttribute('role', 'tooltip');

        // Position near target
        const rect = target.getBoundingClientRect();
        tooltipElement.style.left = rect.left + (rect.width / 2) + 'px';
        tooltipElement.style.top = rect.top - 30 + 'px';

        document.body.appendChild(tooltipElement);

        // Adjust position if off-screen
        const tooltipRect = tooltipElement.getBoundingClientRect();
        if (tooltipRect.left < 10) {
            tooltipElement.style.left = '10px';
        }
        if (tooltipRect.right > window.innerWidth - 10) {
            tooltipElement.style.left = window.innerWidth - tooltipRect.width - 10 + 'px';
        }
    }

    function hideEnhancedTooltip() {
        if (tooltipElement) {
            tooltipElement.remove();
            tooltipElement = null;
        }
    }

    // ===== ANIMATION HELPERS =====
    function addFadeIn(element, duration = 300) {
        element.style.opacity = '0';
        element.style.transition = `opacity ${duration}ms ease-in-out`;

        requestAnimationFrame(() => {
            element.style.opacity = '1';
        });
    }

    function addFadeOut(element, duration = 300) {
        element.style.transition = `opacity ${duration}ms ease-in-out`;
        element.style.opacity = '0';

        setTimeout(() => {
            element.style.display = 'none';
            element.style.opacity = '';
            element.style.transition = '';
        }, duration);
    }

    // ===== UTILITY FUNCTIONS =====
    function debounce(func, wait) {
        let timeout;
        return function executedFunction(...args) {
            const later = () => {
                clearTimeout(timeout);
                func(...args);
            };
            clearTimeout(timeout);
            timeout = setTimeout(later, wait);
        };
    }

    function throttle(func, limit) {
        let inThrottle;
        return function() {
            const args = arguments;
            const context = this;
            if (!inThrottle) {
                func.apply(context, args);
                inThrottle = true;
                setTimeout(() => inThrottle = false, limit);
            }
        };
    }


})();
