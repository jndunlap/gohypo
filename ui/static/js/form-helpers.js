// ===== FORM HELPERS MODULE =====
// Progressive form enhancements and validation

(function() {
    'use strict';

    // Initialize when DOM is ready
    document.addEventListener('DOMContentLoaded', function() {
        console.log('[Form Helpers] Initializing form enhancements...');
        initializeFormValidation();
        initializeFormEnhancements();
    });

    // ===== FORM VALIDATION =====
    function initializeFormValidation() {
        // Progressive validation - enhances but doesn't break server-side validation
        document.addEventListener('blur', function(e) {
            if (e.target.matches('input, textarea, select')) {
                validateField(e.target);
            }
        }, true);

        document.addEventListener('input', function(e) {
            if (e.target.matches('input, textarea')) {
                clearFieldError(e.target);
            }
        });
    }

    function validateField(field) {
        const value = field.value.trim();
        const fieldName = field.name || field.id;
        let isValid = true;
        let errorMessage = '';

        // Required field validation
        if (field.hasAttribute('required') && !value) {
            isValid = false;
            errorMessage = `${fieldName || 'This field'} is required`;
        }

        // Email validation
        else if (field.type === 'email' && value) {
            const emailRegex = /^[^\s@]+@[^\s@]+\.[^\s@]+$/;
            if (!emailRegex.test(value)) {
                isValid = false;
                errorMessage = 'Please enter a valid email address';
            }
        }

        // URL validation
        else if (field.type === 'url' && value) {
            try {
                new URL(value);
            } catch {
                isValid = false;
                errorMessage = 'Please enter a valid URL';
            }
        }

        // Minimum length validation
        else if (field.hasAttribute('minlength') && value) {
            const minLength = parseInt(field.getAttribute('minlength'));
            if (value.length < minLength) {
                isValid = false;
                errorMessage = `${fieldName || 'This field'} must be at least ${minLength} characters`;
            }
        }

        // Maximum length validation
        else if (field.hasAttribute('maxlength') && value) {
            const maxLength = parseInt(field.getAttribute('maxlength'));
            if (value.length > maxLength) {
                isValid = false;
                errorMessage = `${fieldName || 'This field'} must be no more than ${maxLength} characters`;
            }
        }

        // Pattern validation
        else if (field.hasAttribute('pattern') && value) {
            const pattern = new RegExp(field.getAttribute('pattern'));
            if (!pattern.test(value)) {
                isValid = false;
                errorMessage = field.getAttribute('title') || 'Please match the required format';
            }
        }

        // Numeric validation
        else if (field.type === 'number' && value) {
            const numValue = parseFloat(value);
            const min = field.hasAttribute('min') ? parseFloat(field.getAttribute('min')) : null;
            const max = field.hasAttribute('max') ? parseFloat(field.getAttribute('max')) : null;

            if (min !== null && numValue < min) {
                isValid = false;
                errorMessage = `${fieldName || 'This field'} must be at least ${min}`;
            } else if (max !== null && numValue > max) {
                isValid = false;
                errorMessage = `${fieldName || 'This field'} must be no more than ${max}`;
            }
        }

        if (!isValid) {
            showFieldError(field, errorMessage);
        } else {
            clearFieldError(field);
        }

        return isValid;
    }

    function showFieldError(field, message) {
        clearFieldError(field); // Remove any existing error

        const errorDiv = document.createElement('div');
        errorDiv.className = 'field-error text-red-600 text-sm mt-1';
        errorDiv.textContent = message;

        field.classList.add('border-red-500');
        field.parentNode.insertBefore(errorDiv, field.nextSibling);
    }

    function clearFieldError(field) {
        const existingError = field.parentNode.querySelector('.field-error');
        if (existingError) {
            existingError.remove();
        }
        field.classList.remove('border-red-500');
    }

    // ===== FORM ENHANCEMENTS =====
    function initializeFormEnhancements() {
        enhanceFormSubmissions();
    }

    function enhanceFormSubmissions() {
        // Enhanced form submission with loading states
        document.addEventListener('submit', function(e) {
            const form = e.target;
            const submitBtn = form.querySelector('button[type="submit"], input[type="submit"]');

            if (submitBtn) {
                const originalText = submitBtn.textContent || submitBtn.value;

                // Disable button and show loading state
                submitBtn.disabled = true;
                submitBtn.setAttribute('data-original-text', originalText);
                submitBtn.textContent = 'Submitting...';

                // Re-enable after form submission completes
                form.addEventListener('htmx:afterRequest', function() {
                    submitBtn.disabled = false;
                    submitBtn.textContent = submitBtn.getAttribute('data-original-text');
                    submitBtn.removeAttribute('data-original-text');
                }, { once: true });
            }
        });
    }

})();