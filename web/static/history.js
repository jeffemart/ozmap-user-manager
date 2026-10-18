const historyRows = document.getElementById('history-rows');
const historyDate = document.getElementById('history-date');
const historyMessage = document.getElementById('history-message');
const previousPage = document.getElementById('history-previous');
const nextPage = document.getElementById('history-next');
const pageLabel = document.getElementById('history-page');
const historyDetail = document.getElementById('detail');
let currentPage = 1;
let totalPages = 1;

function formatDate(value) {
  return new Intl.DateTimeFormat('pt-BR', {
    dateStyle: 'short',
    timeStyle: 'short'
  }).format(new Date(value));
}

function addCell(row, value) {
  const cell = row.insertCell();
  cell.textContent = value ?? '';
  return cell;
}

async function loadHistory(page = currentPage) {
  historyRows.replaceChildren();
  const loadingRow = historyRows.insertRow();
  const loadingCell = loadingRow.insertCell();
  loadingCell.colSpan = 7;
  loadingCell.textContent = 'Carregando histórico...';
  historyMessage.textContent = '';

  const params = new URLSearchParams({ page: String(page) });
  if (historyDate.value) {
    const start = new Date(`${historyDate.value}T00:00:00`);
    const end = new Date(start);
    end.setDate(end.getDate() + 1);
    params.set('start', start.toISOString());
    params.set('end', end.toISOString());
  }

  try {
    const response = await fetch(`/api/history?${params}`);
    if (!response.ok) throw new Error(await response.text());
    const data = await response.json();
    if (!Array.isArray(data.imports)) throw new Error('Resposta inválida ao carregar o histórico.');

    currentPage = data.page;
    totalPages = data.total_pages;
    historyRows.replaceChildren();
    if (data.imports.length === 0) {
      const row = historyRows.insertRow();
      const cell = row.insertCell();
      cell.colSpan = 7;
      cell.textContent = historyDate.value
        ? 'Nenhuma importação encontrada nesta data.'
        : 'Nenhuma importação registrada.';
    } else {
      for (const item of data.imports) {
        const row = historyRows.insertRow();
        addCell(row, formatDate(item.created_at));
        addCell(row, item.filename);
        addCell(row, item.total);
        addCell(row, item.success_count);
        addCell(row, item.error_count);
        addCell(row, item.status);
        const action = row.insertCell();
        const button = document.createElement('button');
        button.type = 'button';
        button.textContent = 'Detalhes';
        button.dataset.history = item.id;
        button.addEventListener('click', () => loadDetails(item.id));
        action.appendChild(button);
      }
    }

    pageLabel.textContent = `Página ${currentPage} de ${totalPages} — ${data.total} importações`;
    previousPage.disabled = currentPage <= 1;
    nextPage.disabled = currentPage >= totalPages;
  } catch (error) {
    historyRows.replaceChildren();
    const row = historyRows.insertRow();
    const cell = row.insertCell();
    cell.colSpan = 7;
    cell.textContent = 'Não foi possível carregar o histórico.';
    historyMessage.textContent = error.message;
    pageLabel.textContent = '';
    previousPage.disabled = true;
    nextPage.disabled = true;
  }
}

async function loadDetails(id) {
  historyDetail.classList.remove('hidden');
  historyDetail.textContent = 'Carregando detalhes...';
  try {
    const response = await fetch(`/api/history/${encodeURIComponent(id)}`);
    if (!response.ok) throw new Error(await response.text());
    const details = await response.json();
    if (!Array.isArray(details)) throw new Error('Resposta inválida ao carregar os detalhes.');

    historyDetail.replaceChildren();
    const heading = document.createElement('h2');
    heading.textContent = 'Detalhes da importação';
    historyDetail.appendChild(heading);
    if (details.length === 0) {
      const message = document.createElement('p');
      message.textContent = 'Nenhum detalhe de usuário foi registrado para esta importação.';
      historyDetail.appendChild(message);
      return;
    }

    const table = document.createElement('table');
    const header = table.createTHead().insertRow();
    for (const label of ['E-mail', 'Role', 'Status', 'Detalhe']) {
      const cell = document.createElement('th');
      cell.textContent = label;
      header.appendChild(cell);
    }
    const body = table.createTBody();
    for (const detail of details) {
      const row = body.insertRow();
      for (const value of [detail.email, detail.role_name, detail.status, detail.error_message]) {
        addCell(row, value);
      }
      row.className = detail.status === 'created' ? 'ok' : 'error';
    }
    historyDetail.appendChild(table);
    historyDetail.scrollIntoView({ behavior: 'smooth', block: 'start' });
  } catch (error) {
    historyDetail.textContent = `Não foi possível carregar os detalhes: ${error.message}`;
  }
}

document.getElementById('history-filter').addEventListener('submit', event => {
  event.preventDefault();
  loadHistory(1);
});
document.getElementById('clear-history-date').addEventListener('click', () => {
  historyDate.value = '';
  loadHistory(1);
});
previousPage.addEventListener('click', () => loadHistory(currentPage - 1));
nextPage.addEventListener('click', () => loadHistory(currentPage + 1));

loadHistory();
