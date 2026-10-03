// Application dialogs work on desktop, mobile and embedded browsers.
function openDialog(message, {value='',options=null,confirmation=false}={}) {
 return new Promise(resolve=>{
  const dialog=document.createElement('dialog');dialog.className='card';dialog.style.cssText='max-width:min(640px,92vw);width:100%;color:var(--text-primary);background:var(--bg-surface);border:1px solid var(--border-subtle);padding:24px';
  const form=document.createElement('form');form.method='dialog';
  const label=document.createElement('label');label.textContent=message;label.style.display='block';label.id='application-dialog-label';dialog.setAttribute('aria-labelledby',label.id);form.append(label);
  let input;
  if(!confirmation){input=document.createElement(options?'select':'textarea');input.className='input-control';input.setAttribute('aria-label',message);input.style.cssText='display:block;width:100%;margin:16px 0';if(options){for(const option of options){const el=document.createElement('option');el.value=option;el.textContent=option;input.append(el);}}else{input.value=value;input.rows=value.length>200?12:3;}input.required=true;form.append(input);}
  const actions=document.createElement('div');actions.className='flex gap-4';const cancel=document.createElement('button');cancel.type='button';cancel.className='btn btn-outline';cancel.textContent='Cancel';cancel.addEventListener('click',()=>dialog.close());
  const submit=document.createElement('button');submit.type='submit';submit.className='btn btn-primary';submit.textContent=confirmation?'Confirm':'Continue';actions.append(cancel,submit);form.append(actions);dialog.append(form);document.body.append(dialog);
  let result=null;form.addEventListener('submit',event=>{event.preventDefault();result=confirmation?true:input.value;dialog.close();});
  dialog.addEventListener('close',()=>{dialog.remove();resolve(result);},{once:true});dialog.showModal();(input||cancel).focus();
 });
}
export const askText=(message,value='')=>openDialog(message,{value});
export const askSelect=(message,options)=>openDialog(message,{options});
export const confirmAction=async message=>Boolean(await openDialog(message,{confirmation:true}));
