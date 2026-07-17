// Progressive enhancement for <details class="ms"> checkbox dropdowns:
// keep the <summary> label in sync with what's checked, and close on outside click.
// Without JS the checkboxes still work and submit; only the label stays static.
(function () {
  function label(ms) {
    var checked = ms.querySelectorAll('.ms-menu input:checked');
    var summary = ms.querySelector('summary');
    if (!checked.length) { summary.textContent = ms.dataset.all || 'All'; return; }
    if (checked.length > 3) { summary.textContent = checked.length + ' selected'; return; }
    summary.textContent = Array.from(checked, function (c) {
      return c.parentElement.textContent.trim();
    }).join(', ');
  }

  document.querySelectorAll('details.ms').forEach(function (ms) {
    label(ms);
    ms.addEventListener('change', function () { label(ms); });
  });

  document.addEventListener('click', function (e) {
    document.querySelectorAll('details.ms[open]').forEach(function (ms) {
      if (!ms.contains(e.target)) ms.removeAttribute('open');
    });
  });
})();
