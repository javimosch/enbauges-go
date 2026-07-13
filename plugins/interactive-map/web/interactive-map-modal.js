window.InteractiveMapModal = (function() {
  var h = React.createElement;
  var helpers = window.InteractiveMapHelpers;

  function MarkerModal(props) {
    var form = props.form;
    var onChange = props.onChange;
    var onSubmit = props.onSubmit;
    var onClose = props.onClose;
    var submitting = props.submitting;
    var error = props.error;
    var isEdit = props.isEdit;

    var cats = ['lieu', 'service', 'evenement', 'autre'];

    function handleFieldChange(field, value) {
      var updated = Object.assign({}, form);
      if (field.indexOf('.') !== -1) {
        var parts = field.split('.');
        var obj = Object.assign({}, updated[parts[0]]);
        obj[parts[1]] = value;
        updated[parts[0]] = obj;
      } else {
        updated[field] = value;
      }
      onChange(updated);
    }

    function handleCategorySelect(cat) {
      handleFieldChange('category', cat);
    }

    return h('div', { className: 'imap-modal-overlay', onClick: function(e) { if (e.target === e.currentTarget) onClose(); } },
      h('div', { className: 'imap-modal' },
        h('div', { className: 'imap-modal__header' },
          h('span', { className: 'imap-modal__title' }, isEdit ? 'Modifier le marqueur' : 'Ajouter un marqueur'),
          h('button', { className: 'imap-modal__close', onClick: onClose, 'aria-label': 'Fermer' },
            h('svg', { width: 20, height: 20, viewBox: '0 0 20 20', fill: 'currentColor' },
              h('path', { d: 'M6 6L14 14M14 6L6 14', stroke: 'currentColor', strokeWidth: 2, strokeLinecap: 'round' })
            )
          )
        ),
        h('div', { className: 'imap-modal__body' },
          // Category
          h('div', { className: 'imap-field' },
            h('label', { className: 'imap-field__label' }, 'Catégorie *'),
            h('div', { className: 'imap-cat-toggle' },
              cats.map(function(cat) {
                return h('button', {
                  key: cat,
                  'data-cat': cat,
                  className: 'imap-cat-toggle__btn' + (form.category === cat ? ' imap-cat-toggle__btn--active' : ''),
                  onClick: function() { handleCategorySelect(cat); },
                  type: 'button'
                }, helpers.getCategoryIcon(cat) + ' ' + helpers.getCategoryLabel(cat));
              })
            )
          ),

          // Title
          h('div', { className: 'imap-field' },
            h('label', { className: 'imap-field__label' }, 'Titre *'),
            h('input', {
              className: 'imap-field__input',
              type: 'text',
              value: form.title,
              maxLength: 100,
              placeholder: 'Nom du lieu, service, événement...',
              onChange: function(e) { handleFieldChange('title', e.target.value); }
            }),
            h('div', { className: 'imap-field__counter' }, (form.title || '').length + '/100')
          ),

          // Description
          h('div', { className: 'imap-field' },
            h('label', { className: 'imap-field__label' }, 'Description'),
            h('textarea', {
              className: 'imap-field__input',
              value: form.description || '',
              maxLength: 500,
              rows: 3,
              placeholder: 'Décrivez brièvement ce lieu ou service...',
              onChange: function(e) { handleFieldChange('description', e.target.value); },
              style: { resize: 'vertical' }
            }),
            h('div', { className: 'imap-field__counter' }, (form.description || '').length + '/500')
          ),

          // Address (read-only, derived from map click)
          h('div', { className: 'imap-field' },
            h('label', { className: 'imap-field__label' }, 'Adresse'),
            h('input', {
              className: 'imap-field__input',
              type: 'text',
              value: form.location.address || '',
              maxLength: 300,
              placeholder: 'Adresse (remplie automatiquement)',
              onChange: function(e) { handleFieldChange('location.address', e.target.value); }
            })
          ),

          // Contact
          h('div', { className: 'imap-field' },
            h('label', { className: 'imap-field__label' }, 'Contact'),
            h('input', {
              className: 'imap-field__input',
              type: 'text',
              value: form.contact || '',
              maxLength: 200,
              placeholder: 'Email ou téléphone (optionnel)',
              onChange: function(e) { handleFieldChange('contact', e.target.value); }
            })
          ),

          // Tags
          h('div', { className: 'imap-field' },
            h('label', { className: 'imap-field__label' }, 'Tags'),
            h('input', {
              className: 'imap-field__input',
              type: 'text',
              value: form.tagsInput || '',
              placeholder: 'Séparés par des virgules (ex: collaboratif, numérique)',
              onChange: function(e) { handleFieldChange('tagsInput', e.target.value); }
            })
          ),

          // Location info
          form.location.lat && form.location.lng ? h('div', {
            style: {
              background: 'var(--enbauges-cream)',
              borderRadius: '0.5rem',
              padding: '0.5rem 0.75rem',
              fontSize: '0.8125rem',
              color: 'var(--enbauges-sage)',
              marginBottom: '1rem',
              display: 'flex',
              alignItems: 'center',
              gap: '0.375rem'
            }
          },
            h('svg', { width: 14, height: 14, viewBox: '0 0 14 14', fill: 'currentColor' },
              h('path', { d: 'M7 1C4.79 1 3 2.79 3 5c0 3.5 4 8 4 8s4-4.5 4-8c0-2.21-1.79-4-4-4zm0 6.5A2.5 2.5 0 1 1 7 2.5a2.5 2.5 0 0 1 0 5z' })
            ),
            h('span', null, form.location.lat.toFixed(4) + ', ' + form.location.lng.toFixed(4))
          ) : null,

          // Error
          error ? h('div', { style: { color: '#DC2626', fontSize: '0.875rem', marginBottom: '0.75rem' } }, error) : null
        ),
        h('div', { className: 'imap-modal__footer' },
          h('button', {
            className: 'enbauges-btn-secondary',
            onClick: onClose,
            type: 'button'
          }, 'Annuler'),
          h('button', {
            className: 'enbauges-btn-primary',
            onClick: onSubmit,
            disabled: submitting,
            type: 'button'
          }, submitting ? 'Envoi…' : (isEdit ? 'Modifier' : 'Ajouter'))
        )
      )
    );
  }

  return MarkerModal;
})();
