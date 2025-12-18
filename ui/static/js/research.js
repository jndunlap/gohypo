/**
 * Research UI JavaScript - HTMX Event Handling and UI Updates
 * Handles real-time updates for the live discovery command interface
 */

(function() {
    'use strict';

    // Initialize when DOM is ready
    document.addEventListener('DOMContentLoaded', function() {
        initializeResearchUI();
        setupHTMXListeners();
        startProgressPolling();
    });

    /**
     * Initialize research UI components
     */
    function initializeResearchUI() {
        console.log('[Research UI] Initializing...');
        
        // Set up modal close handlers
        setupModalHandlers();
        
        // Initialize empty states
        initializeEmptyStates();
    }

    /**
     * Set up HTMX event listeners for research updates
     */
    function setupHTMXListeners() {
        // Listen for research progress updates
        document.body.addEventListener('researchProgressUpdate', function(evt) {
            console.log('[Research UI] Progress update:', evt.detail);
            updateProgressBar(evt.detail);
        });

        // Listen for new hypotheses
        document.body.addEventListener('hypothesisValidated', function(evt) {
            console.log('[Research UI] New hypothesis:', evt.detail);
            addHypothesisToLedger(evt.detail);
            updateHypothesisCount();
        });

        // Listen for scan completion
        document.body.addEventListener('researchComplete', function(evt) {
            console.log('[Research UI] Research complete:', evt.detail);
            showCompletionState();
            enableDownloadButton();
        });

        // Listen for HTMX afterSwap to trigger custom events
        document.body.addEventListener('htmx:afterSwap', function(evt) {
            if (evt.detail.target.id === 'research-ledger-container') {
                // Trigger custom event for research update
                document.body.dispatchEvent(new CustomEvent('researchUpdate'));
            }
        });

        // Listen for HTMX afterRequest to update UI state
        document.body.addEventListener('htmx:afterRequest', function(evt) {
            if (evt.detail.pathInfo.requestPath.includes('/api/research/initiate')) {
                // Research initiated, start polling
                startProgressPolling();
            }
        });
    }

    /**
     * Update progress bar with new data
     */
    function updateProgressBar(data) {
        const fill = document.getElementById('progress-fill');
        const text = document.getElementById('progress-text');
        
        if (fill) {
            fill.style.width = data.progress + '%';
        }
        
        if (text) {
            text.textContent = data.status || 'Processing...';
        }
    }

    /**
     * Add new hypothesis to ledger (prepend)
     */
    function addHypothesisToLedger(hypothesis) {
        const ledger = document.getElementById('research-ledger');
        if (!ledger) return;

        // Create card HTML (simplified - actual rendering should come from server)
        const cardHTML = generateHypothesisCardHTML(hypothesis);
        
        // Prepend new card
        if (ledger.querySelector('.empty-ledger')) {
            ledger.innerHTML = cardHTML;
        } else {
            ledger.insertAdjacentHTML('afterbegin', cardHTML);
        }
        
        // Animate new card
        const newCard = ledger.querySelector('[data-hypothesis-id="' + hypothesis.id + '"]');
        if (newCard) {
            newCard.style.opacity = '0';
            newCard.style.transform = 'translateY(-10px)';
            setTimeout(() => {
                newCard.style.transition = 'all 0.3s ease';
                newCard.style.opacity = '1';
                newCard.style.transform = 'translateY(0)';
            }, 10);
        }
    }

    /**
     * Generate hypothesis row HTML (fallback if server doesn't provide)
     * Note: This is a simplified fallback - actual rendering should come from server templates
     */
    function generateHypothesisCardHTML(hypothesis) {
        const validated = hypothesis.validated ? 'validated' : 'rejected';
        const statusIcon = hypothesis.validated ? '✓' : '✗';
        const claim = hypothesis.claim || 'No claim available';
        const truncatedClaim = claim.length > 80 ? claim.substring(0, 77) + '...' : claim;
        
        return `
            <tr class="hover:bg-gray-50 ${validated === 'validated' ? 'bg-green-50' : 'bg-red-50'}" data-hypothesis-id="${hypothesis.id}">
                <td class="px-6 py-4 whitespace-nowrap">
                    <span class="inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium ${validated === 'validated' ? 'bg-green-100 text-green-800' : 'bg-red-100 text-red-800'}">${statusIcon} ${validated}</span>
                </td>
                <td class="px-6 py-4">
                    <div class="text-sm font-medium text-gray-900">${escapeHtml(truncatedClaim)}</div>
                    <div class="text-xs text-gray-500 mt-1">${escapeHtml(hypothesis.id)}</div>
                </td>
                <td class="px-6 py-4 whitespace-nowrap text-center">—</td>
                <td class="px-6 py-4 whitespace-nowrap text-center">—</td>
                <td class="px-6 py-4 whitespace-nowrap text-center">—</td>
                <td class="px-6 py-4 whitespace-nowrap text-center">
                    <button class="text-sm text-blue-600 hover:text-blue-800 font-medium" onclick="toggleEvidenceDrawer('${hypothesis.id}')">Details</button>
                </td>
            </tr>
        `;
    }

    /**
     * Update hypothesis count in control strip
     */
    function updateHypothesisCount() {
        const ledger = document.getElementById('research-ledger');
        const countElement = document.getElementById('hypothesis-count');
        
        if (ledger && countElement) {
            // Count table rows with data-hypothesis-id attribute (excluding drawer rows)
            const count = ledger.querySelectorAll('tr[data-hypothesis-id]').length;
            countElement.textContent = count;
        }
    }

    /**
     * Show completion state
     */
    function showCompletionState() {
        const btn = document.getElementById('initiate-scan-btn');
        if (btn) {
            btn.innerHTML = '<span class="btn-text">✅ COMPLETE</span>';
            btn.classList.add('success');
            btn.classList.remove('disabled');
        }
    }

    /**
     * Enable download button
     */
    function enableDownloadButton() {
        const btn = document.getElementById('download-btn');
        if (btn) {
            btn.classList.remove('disabled');
            btn.removeAttribute('disabled');
        }
    }

    /**
     * Start polling for progress updates
     */
    function startProgressPolling() {
        // Check if polling is already active
        if (window.researchPollingInterval) {
            return;
        }

        // Poll every 2 seconds for status updates
        window.researchPollingInterval = setInterval(function() {
            const statusElement = document.getElementById('research-status');
            if (statusElement) {
                // Trigger HTMX request to update status
                htmx.ajax('GET', '/api/research/status', {
                    target: '#research-status',
                    swap: 'innerHTML'
                });
            }
        }, 2000);
    }

    /**
     * Stop progress polling
     */
    function stopProgressPolling() {
        if (window.researchPollingInterval) {
            clearInterval(window.researchPollingInterval);
            window.researchPollingInterval = null;
        }
    }

    /**
     * Set up modal handlers
     */
    function setupModalHandlers() {
        // Close modal on backdrop click
        const backdrop = document.querySelector('.modal-backdrop');
        if (backdrop) {
            backdrop.addEventListener('click', closeEvidenceModal);
        }

        // Close modal on ESC key
        document.addEventListener('keydown', function(evt) {
            if (evt.key === 'Escape') {
                closeEvidenceModal();
            }
        });
    }

    /**
     * Close evidence modal
     */
    window.closeEvidenceModal = function() {
        const modal = document.getElementById('evidence-detail-modal');
        if (modal) {
            modal.style.display = 'none';
        }
    };

    /**
     * Open evidence modal
     */
    window.openEvidenceModal = function() {
        const modal = document.getElementById('evidence-detail-modal');
        if (modal) {
            modal.style.display = 'flex';
        }
    };

    /**
     * Initialize empty states
     */
    function initializeEmptyStates() {
        // Check if ledger is empty and show appropriate message
        const ledger = document.getElementById('research-ledger');
        if (ledger && ledger.querySelectorAll('tr[data-hypothesis-id]').length === 0) {
            // Empty state already handled by template
        }
    }

    /**
     * Toggle evidence drawer for a hypothesis (table row expand/collapse)
     */
    window.toggleEvidenceDrawer = function(hypothesisId) {
        const drawer = document.getElementById('drawer-' + hypothesisId);
        if (!drawer) return;

        const isVisible = drawer.style.display !== 'none';
        drawer.style.display = isVisible ? 'none' : 'table-row';
    };

    /**
     * Escape HTML to prevent XSS
     */
    function escapeHtml(text) {
        const div = document.createElement('div');
        div.textContent = text;
        return div.innerHTML;
    }

    /**
     * Format relative time
     */
    function formatRelativeTime(date) {
        const now = new Date();
        const diff = now - new Date(date);
        const seconds = Math.floor(diff / 1000);
        const minutes = Math.floor(seconds / 60);
        const hours = Math.floor(minutes / 60);
        const days = Math.floor(hours / 24);

        if (days > 0) return days + ' day' + (days > 1 ? 's' : '') + ' ago';
        if (hours > 0) return hours + ' hour' + (hours > 1 ? 's' : '') + ' ago';
        if (minutes > 0) return minutes + ' minute' + (minutes > 1 ? 's' : '') + ' ago';
        return 'just now';
    }

    // Export functions for global access
    window.researchUI = {
        updateProgressBar: updateProgressBar,
        addHypothesisToLedger: addHypothesisToLedger,
        updateHypothesisCount: updateHypothesisCount,
        showCompletionState: showCompletionState,
        enableDownloadButton: enableDownloadButton,
        startProgressPolling: startProgressPolling,
        stopProgressPolling: stopProgressPolling,
        closeEvidenceModal: closeEvidenceModal,
        openEvidenceModal: openEvidenceModal
    };

})();

