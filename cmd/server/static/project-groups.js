const csrf = document.querySelector('meta[name="csrf-token"]').content;
document.getElementById('project-group-form').addEventListener('submit', async event => {
  event.preventDefault();
  const body = {
    name: document.getElementById('project-group-name').value.trim(),
    id: document.getElementById('project-group-id').value.trim()
  };
  const response = await fetch('/api/project-groups', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json', 'X-CSRF-Token': csrf },
    body: JSON.stringify(body)
  });
  if (!response.ok) {
    alert(await response.text());
    return;
  }
  location.reload();
});

document.querySelectorAll('[data-delete]').forEach(button => button.addEventListener('click', async () => {
  if (!confirm('Excluir este grupo de projetos?')) return;
  const response = await fetch(`/api/project-groups?id=${encodeURIComponent(button.dataset.delete)}`, {
    method: 'DELETE',
    headers: { 'X-CSRF-Token': csrf }
  });
  if (!response.ok) alert(await response.text());
  else location.reload();
}));
