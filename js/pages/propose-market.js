import '../components/sidebar.js';
import '../components/topbar.js';
import ApiClient from '../api/client.js';
import { showToast } from '../components/toast.js';

document.addEventListener('DOMContentLoaded', () => {
    if (!ApiClient || !ApiClient.isAuthenticated()) {
        window.location.href = 'login.html';
        return;
    }
});

window.submitProposal = async (e) => {
    e.preventDefault();

    const title = document.getElementById('pTitle').value;
    const category = document.getElementById('pCategory').value;
    const description = document.getElementById('pDescription').value;
    const source = document.getElementById('pSource').value;
    const lockTime = document.getElementById('pLockTime').value;

    if (!title || !category || !description || !source || !lockTime) {
        showToast("Please fill all required fields.", "error");
        return;
    }

    const lockDate = new Date(lockTime);
    if (lockDate <= new Date()) {
        showToast("Lock time must be in the future.", "error");
        return;
    }

    const payload = {
        title: title,
        category: category,
        description: description,
        resolution_source: source,
        lock_time: lockDate.toISOString(),
        options: document.getElementById('pOptions').value,
        difficulty:document.getElementById('pDifficulty').value,
        range_width:Number(document.getElementById('pRangeWidth').value),
        resolution_rule:description,
    };

    const btn = document.getElementById('submitBtn');
    btn.disabled = true;
    btn.innerHTML = '<i class="ph ph-spinner ph-spin"></i> Submitting...';

    try {
        await ApiClient.post('/markets/propose', payload);
        showToast("Market proposal submitted successfully! Redirecting to dashboard...", "success");
        setTimeout(() => {
            window.location.href = 'dashboard.html';
        }, 2000);
    } catch (err) {
        showToast(err.message, "error");
        btn.disabled = false;
        btn.innerHTML = 'Submit Proposal for Review';
    }
};

const formats={Weather:['Binary rain: 20 coins','Temperature range: 50 coins','Exact temperature: 120 coins'],Sports:['Winner: 25 coins','Run/score range: 60 coins','Exact score or run total: 200 coins'],Politics:['Winner: 30 coins','Margin within 5 percentage points: 80 coins','Exact seat count: 250 coins'],Entertainment:['Winner: 25 coins','Exactly three nominees: 70 coins','Collection range: 150 coins'],'Financial Markets':['Direction: 20 coins','Percentage-change guess within ±1 percentage point: 80 coins','Closest price: 300 coins'],'Wild Card':['Two options: 40 coins','Four options: 100 coins','Closest measurable numeric answer: 400 coins']};
function describeFormat(){const category=document.getElementById('pCategory').value;const difficulty=document.getElementById('pDifficulty').value;document.getElementById('predictionFormat').textContent=(formats[category]||formats['Financial Markets'])[['Easy','Medium','Hard'].indexOf(difficulty)]+'. Specify the exact outcome, units, observation time and approved source in the resolution criteria.';}
document.getElementById('pCategory').addEventListener('change',describeFormat);document.getElementById('pDifficulty').addEventListener('change',describeFormat);describeFormat();
