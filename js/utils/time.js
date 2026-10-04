export function age(date, now=Date.now()) {
    const minutes=Math.floor((now-new Date(date).getTime())/60000);
    return !Number.isFinite(minutes)||minutes<0?'Time unavailable':minutes<1?'Just published':minutes<60?`${minutes} min ago`:`${Math.floor(minutes/60)} hr ago`;
}
export function remaining(date, now=Date.now()) {
    const minutes=Math.ceil((new Date(date).getTime()-now)/60000);
    return !Number.isFinite(minutes)||minutes<=0?'Closed':minutes<60?`${minutes} min left`:minutes>=2880?`${Math.floor(minutes/1440)} days left`:`${Math.floor(minutes/60)} hr ${minutes%60} min left`;
}
