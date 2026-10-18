const csrf = document.querySelector('meta[name="csrf-token"]').content;
let importId = null;
let valid = 0;
let users = [];
let currentPage = 1;
const pageSize = 20;
const file = document.getElementById('file');
const upload = document.getElementById('upload');
const preview = document.getElementById('preview');
const create = document.getElementById('create');
const rows = document.getElementById('rows');
const pageLabel = document.getElementById('import-page');
const previousPage = document.getElementById('import-previous');
const nextPage = document.getElementById('import-next');
const resultRows = document.getElementById('result-rows');
const resultPageLabel = document.getElementById('result-page');
const resultPrevious = document.getElementById('result-previous');
const resultNext = document.getElementById('result-next');
const resultPageSize = 20;
let resultItems = [];
let resultCurrentPage = 1;

function renderUsersPage() {
  const totalPages = Math.max(1, Math.ceil(users.length / pageSize));
  currentPage = Math.min(Math.max(currentPage, 1), totalPages);
  rows.replaceChildren();

  for (const user of users.slice((currentPage - 1) * pageSize, currentPage * pageSize)) {
    const tr = document.createElement('tr');
    const resources = Array.isArray(user.resources) ? user.resources.join(', ') : '';
    const projectGroups = Array.isArray(user.projectGroups) ? user.projectGroups.join(', ') : '';
    const errors = Array.isArray(user.errors) ? user.errors.join('; ') : '';
    for (const value of [user.row, user.email, user.role, resources, projectGroups, user.valid ? 'OK' : 'ERRO', errors]) {
      const td = document.createElement('td');
      td.textContent = value ?? '';
      tr.appendChild(td);
    }
    rows.appendChild(tr);
  }

  pageLabel.textContent = `Página ${currentPage} de ${totalPages} — ${users.length} usuários`;
  previousPage.disabled = currentPage === 1;
  nextPage.disabled = currentPage === totalPages;
}

upload.onclick = async () => {
  if (!file.files[0]) return alert('Selecione um XLSX.');

  upload.disabled = true;
  let r;
  try {
    const fd = new FormData();
    fd.append('file', file.files[0]);
    r = await fetch('/api/upload', {
      method: 'POST',
      headers: { 'X-CSRF-Token': csrf },
      body: fd
    });
  } catch (error) {
    alert(`Não foi possível enviar a planilha: ${error.message}`);
    return;
  } finally {
    upload.disabled = false;
  }
  if (!r.ok) return alert(await r.text());

  const d = await r.json();
  if (!Array.isArray(d.users)) {
    return alert('Resposta inválida do servidor: lista de usuários ausente.');
  }

  importId = d.import_id;
  valid = d.valid;
  users = d.users;
  currentPage = 1;
  preview.classList.remove('hidden');
  document.getElementById('summary-total').textContent = d.total;
  document.getElementById('summary-valid').textContent = d.valid;
  document.getElementById('summary-invalid').textContent = d.invalid;
  renderUsersPage();
  create.disabled = valid === 0;
};

previousPage.addEventListener('click', () => {
  currentPage--;
  renderUsersPage();
});
nextPage.addEventListener('click', () => {
  currentPage++;
  renderUsersPage();
});

function renderResultsPage() {
  const totalPages = Math.max(1, Math.ceil(resultItems.length / resultPageSize));
  resultCurrentPage = Math.min(Math.max(resultCurrentPage, 1), totalPages);
  resultRows.replaceChildren();
  for (const item of resultItems.slice((resultCurrentPage - 1) * resultPageSize, resultCurrentPage * resultPageSize)) {
    const row = document.createElement('tr');
    for (const value of [item.email, item.roleName, item.status, item.message]) {
      const cell = document.createElement('td');
      cell.textContent = value ?? '';
      row.appendChild(cell);
    }
    row.className = item.status === 'created' ? 'ok' : 'error';
    resultRows.appendChild(row);
  }
  resultPageLabel.textContent = `Página ${resultCurrentPage} de ${totalPages} — ${resultItems.length} resultados`;
  resultPrevious.disabled = resultCurrentPage === 1;
  resultNext.disabled = resultCurrentPage === totalPages;
}

resultPrevious.addEventListener('click', () => {
  resultCurrentPage--;
  renderResultsPage();
});
resultNext.addEventListener('click', () => {
  resultCurrentPage++;
  renderResultsPage();
});

create.onclick = async () => {
  if (!importId || !confirm(`Criar ${valid} usuários no OZmap?`)) return;
  create.disabled = true;
  const box = document.getElementById('result');
  box.classList.remove('hidden');
  document.getElementById('result-heading').textContent = 'Criando usuários';
  document.getElementById('create-error').classList.add('hidden');
  document.getElementById('result-table-wrap').classList.add('hidden');
  document.getElementById('result-pagination').classList.add('hidden');
  document.getElementById('create-progress').value = 0;
  document.getElementById('create-progress-text').textContent = 'Iniciando processamento...';

  try {
    const response = await fetch('/api/create', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json', 'X-CSRF-Token': csrf },
      body: JSON.stringify({ import_id: importId })
    });
    if (!response.ok) throw new Error(await response.text());

    let progress;
    do {
      const statusResponse = await fetch(`/api/create/${encodeURIComponent(importId)}`);
      if (!statusResponse.ok) throw new Error(await statusResponse.text());
      progress = await statusResponse.json();
      const percent = progress.total > 0 ? Math.round((progress.processed / progress.total) * 100) : 0;
      document.getElementById('create-progress').value = percent;
      document.getElementById('create-progress-text').textContent =
        `${progress.processed} de ${progress.total} processados — ${progress.created} criados — ${progress.failed} com erro`;
      if (!progress.complete) await new Promise(resolve => setTimeout(resolve, 700));
    } while (!progress.complete);

    if (progress.error) {
      document.getElementById('result-heading').textContent = 'Não foi possível concluir a criação';
      const error = document.getElementById('create-error');
      error.textContent = progress.error;
      error.classList.remove('hidden');
    } else {
      document.getElementById('result-heading').textContent = 'Resultado da criação';
    }
    resultItems = Array.isArray(progress.results) ? progress.results : [];
    resultCurrentPage = 1;
    if (resultItems.length > 0) {
      renderResultsPage();
      document.getElementById('result-table-wrap').classList.remove('hidden');
      document.getElementById('result-pagination').classList.remove('hidden');
    }
  } catch (error) {
    const errorBox = document.getElementById('create-error');
    errorBox.textContent = `Falha ao acompanhar a criação: ${error.message}`;
    errorBox.classList.remove('hidden');
  }
};
