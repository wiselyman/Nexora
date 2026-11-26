const $=id=>document.getElementById(id);let formRevision=0;let savedRevision=0;const dirtyFields=new Set();let approvalSignature='';let approvalPollRunning=false;let approvalRequestIDs=new Set();let configLoadPromise=null;
let setActiveTab=()=>{};
let savePromise=null;
const uiSessionKey='Nexora-ui-session';
const startupToken=new URLSearchParams(location.hash.slice(1)).get('ui_session')||'';
let uiToken=startupToken;
try{if(startupToken){sessionStorage.setItem(uiSessionKey,startupToken);history.replaceState(null,'',location.pathname+location.search)}else{uiToken=sessionStorage.getItem(uiSessionKey)||''}}catch{}
async function api(path,body){const headers={'X-Nexora-UI-Session':uiToken};if(body)headers['Content-Type']='application/json';const res=await fetch(path,{method:body?'POST':'GET',headers,body:body?JSON.stringify(body):undefined});if(!res.ok){let m=await res.text();if(res.status===401){uiToken='';try{sessionStorage.removeItem(uiSessionKey)}catch{}}throw Error(m||`请求失败 (${res.status})`)}return res.json()}
function toast(s){$('toast').textContent=s;$('toast').classList.add('show');setTimeout(()=>$('toast').classList.remove('show'),2700)}
function setTextIfChanged(id,value){const element=$(id);if(element.textContent!==value)element.textContent=value}
function setupTabs(){const tabs=[...document.querySelectorAll('[role="tab"]')];const activate=tab=>{for(const candidate of tabs){const selected=candidate===tab;candidate.setAttribute('aria-selected',String(selected));candidate.tabIndex=selected?0:-1;candidate.classList.toggle('active',selected);const panel=$(candidate.getAttribute('aria-controls'));panel.hidden=!selected;panel.classList.toggle('active',selected)}};setActiveTab=id=>{const tab=$(id);if(tab)activate(tab)};for(const tab of tabs){tab.addEventListener('click',()=>activate(tab));tab.addEventListener('keydown',event=>{let index=tabs.indexOf(tab);if(event.key==='ArrowRight')index=(index+1)%tabs.length;else if(event.key==='ArrowLeft')index=(index+tabs.length-1)%tabs.length;else if(event.key==='Home')index=0;else if(event.key==='End')index=tabs.length-1;else return;event.preventDefault();activate(tabs[index]);tabs[index].focus()})}for(const button of document.querySelectorAll('[data-open-tab]'))button.addEventListener('click',()=>{setActiveTab(button.dataset.openTab);$(button.dataset.openTab)?.focus()})}
function updateSaveStatus(){const dirty=formRevision!==savedRevision;$('saveStatus').textContent=dirty?'有未保存更改':'配置已保存'}
function markFormDirty(id){dirtyFields.add(id);formRevision++;updateSaveStatus()}
async function loadConfig(){try{const c=await api('/api/config');const values={tunnelId:c.tunnel_id||'',clientPath:c.tunnel_client||'tunnel-client',fileRoots:(c.file_roots||[]).join('\n'),allowCommands:!!c.allow_commands,allowFiles:!!c.allow_files,allowDesktop:!!c.allow_desktop,allowBrowser:!!c.allow_browser,allowAdmin:!!c.allow_admin,startOnLaunch:!!c.start_on_launch};for(const [id,value] of Object.entries(values)){if(dirtyFields.has(id))continue;const field=$(id);if(field.type==='checkbox')field.checked=value;else field.value=value}if(dirtyFields.size===0)savedRevision=formRevision;updateSaveStatus();return true}catch(e){$('saveStatus').textContent='无法读取当前配置';toast(e.message);return false}}
function clearFormError(){const box=$('formError');box.textContent='';box.classList.add('hidden');for(const id of ['tunnelId','apiKey','clientPath','fileRoots']){const field=$(id);field.removeAttribute('aria-invalid');field.setAttribute('aria-describedby',(field.getAttribute('aria-describedby')||'').split(/\s+/).filter(v=>v&&v!=='formError').join(' '))}}
function showFormError(message){const box=$('formError');box.textContent=message;box.classList.remove('hidden');let target=null;if(/Tunnel ID.*运行时 API Key/i.test(message))target=$('tunnelId').value.trim()? $('apiKey'):$('tunnelId');else if(/tunnel id|tunnel_id/i.test(message))target=$('tunnelId');else if(/runtime api key|运行时 api key|运行时密钥/i.test(message))target=$('apiKey');else if(/file|folder|文件夹|文件访问/i.test(message))target=$('fileRoots');else if(/tunnel[- ]client|客户端路径|客户端版本|无法读取 tunnel-client/i.test(message))target=$('clientPath');if(target){setActiveTab(target.id==='fileRoots'?'tabPermissions':'tabConnection');target.setAttribute('aria-invalid','true');target.setAttribute('aria-describedby',[target.getAttribute('aria-describedby'),'formError'].filter(Boolean).join(' '));target.focus()}else box.focus()}
function setSavingConfig(saving){$('saveConfigButton').disabled=saving;$('saveChangesButton').disabled=saving;$('connectBtn').disabled=saving}
function saveConfig(silent=false){if(savePromise)return savePromise;setSavingConfig(true);savePromise=saveConfigOnce(silent).finally(()=>{savePromise=null;setSavingConfig(false)});return savePromise}
async function saveConfigOnce(silent){
  if(!configLoadPromise)configLoadPromise=loadConfig();
  if(!await configLoadPromise){showFormError('无法读取当前配置，未保存任何更改。请刷新页面后重试。');return false}
  clearFormError();
  const revision=formRevision;
  const apiKeyValue=$('apiKey').value;
  try{
    const config={name:'Nexora',tunnel_id:$('tunnelId').value.trim(),tunnel_client:$('clientPath').value.trim()||'tunnel-client',file_roots:$('fileRoots').value.split(/\r?\n/).map(v=>v.trim()).filter(Boolean),allow_commands:$('allowCommands').checked,allow_files:$('allowFiles').checked,allow_desktop:$('allowDesktop').checked,allow_browser:$('allowBrowser').checked,allow_admin:$('allowAdmin').checked,start_on_launch:$('startOnLaunch').checked};
    const payload={config};
    if(apiKeyValue)payload.runtime_api_key=apiKeyValue;
    await api('/api/config',payload);
    savedRevision=revision;
    if(formRevision===revision)dirtyFields.clear();
    if(apiKeyValue&&$('apiKey').value===apiKeyValue){$('apiKey').value='';$('keyState').textContent='已保存'}
    updateSaveStatus();
    if(!silent)toast('配置已保存');
    await refresh();
    return true;
  }catch(e){
    $('saveStatus').textContent='更改尚未保存';
    showFormError(e.message);
    toast(e.message);
    return false;
  }
}
async function selectFileRoot(){try{const r=await api('/api/file-roots/select',{});const paths=$('fileRoots').value.split(/\r?\n/).map(v=>v.trim()).filter(Boolean);if(!paths.includes(r.path)){paths.push(r.path);$('fileRoots').value=paths.join('\n');markFormDirty('fileRoots')}toast('已添加文件夹，保存配置后生效')}catch(e){toast(e.message)}}
async function selectTunnelClient(){try{const r=await api('/api/tunnel-client/select',{});$('clientPath').value=r.path;markFormDirty('clientPath');toast('已选择客户端；保存配置后生效')}catch(e){toast(e.message)}}
async function connect(){if(!await saveConfig(true))return;if(formRevision!==savedRevision){toast('保存期间配置又有修改；请先保存更改，再启动 Tunnel');return}try{await api('/api/start',{});toast('Tunnel 启动中');await refresh()}catch(e){showFormError(e.message);toast(e.message);await refresh()}}
async function stopTunnel(){try{await api('/api/stop',{});toast('已请求停止 Tunnel');setTimeout(refresh,500)}catch(e){toast(e.message)}}
async function quitApp(){if(!window.confirm('退出 Nexora 会断开 Tunnel，并停止等待中的命令。之后可重新打开 Nexora。确定退出吗？'))return;const button=$('quitAppButton');button.disabled=true;try{await api('/api/quit',{});toast('Nexora 正在退出');button.textContent='正在退出…'}catch(e){button.disabled=false;toast(e.message)}}
function recoveryAdvice(error){
  const e=(error||'').toLowerCase();
  if(/找不到 tunnel-client|no such file|cannot find/.test(e))return'请从 OpenAI 官方发布页下载与你的系统和芯片架构相符的 tunnel-client，点击“选择文件”填写路径；若已加入系统 PATH，可保留默认名称。';
  if(/版本过旧|版本超时|无法读取 tunnel-client 版本|无法识别 tunnel-client 版本|version.*(old|unsupported|unrecognized|timeout)/.test(e))return'当前 tunnel-client 版本过旧或无法识别。请从官方发布页下载 v0.0.17 或更高版本，点击“选择文件”更新路径，保存后重试。';
  if(/(?:127\.0\.0\.1:5060|localhost:5060).{0,160}(?:401|403|unauthorized|forbidden)|(?:401|403|unauthorized|forbidden).{0,160}(?:127\.0\.0\.1:5060|localhost:5060)|(?:invalid|missing|unauthorized).{0,60}x-5060-token|x-5060-token.{0,60}(?:invalid|missing|unauthorized)/.test(e))return'本机 MCP 鉴权未通过。请退出并重新启动 Nexora，再启动 Tunnel；不要把本机令牌手工粘贴到 ChatGPT 或分享给他人。';
  if(/401|403|unauthorized|forbidden|tunnels read|tunnels use|runtime api key|control.plane.*auth/.test(e))return'请确认 Tunnel ID 和组织选择正确，并确认运行时 API Key 对该 Tunnel 有 Tunnels Read + Use 权限；不要使用管理员密钥。修正后停止并重新启动 Tunnel。';
  if(/dns|network|connection refused|connection reset|timeout|tls|certificate|proxy|no route to host/.test(e))return'连接可能被网络、代理、防火墙或证书设置阻挡。检查网络能否访问 OpenAI Tunnel；公司网络请确认代理和证书配置，再查看运行日志重试。';
  if(/address already in use|bind:|listen tcp/.test(e))return'本机服务端口 5060 可能已被占用。若 Nexora 已运行，请使用启动终端输出的设置链接；否则关闭占用该端口的程序后重试。';
  if(/permission denied|access is denied|operation not permitted/.test(e))return'系统拒绝执行该客户端。请确认选择的是 OpenAI 官方 tunnel-client 可执行文件，并检查当前账户是否有权限运行；必要时重新下载对应系统和架构的版本。';
  if(/意外退出|unexpected.*exit/.test(e))return'Tunnel 客户端已自行退出，即使退出码为 0 也不会保持连接。查看最近运行日志中的最后几行，确认 Tunnel ID、网络和客户端配置后，点击“启动 Tunnel”重试。';
  if(/exit status|signal:|exited/.test(e))return'客户端已退出。展开运行日志查看具体错误；修正 Tunnel ID、运行密钥、客户端路径或网络问题后重试。';
  return'查看运行日志中的第一条错误，核对 Tunnel ID、运行时密钥权限、客户端版本和网络连接后重试。';
}
function updateSetupGuide(s){const states=[$('setupStep1'),$('setupStep2'),$('setupStep3')];const labels=[$('setupStep1State'),$('setupStep2State'),$('setupStep3State')];const configured=!!s.tunnel_configured;const ready=!!s.tunnel_ready;const running=!!s.tunnel_running;const current=configured?(ready?2:1):0;const status=[configured?'已完成':'现在开始',ready?'已完成':running?'正在连接':configured?(s.last_error?'修复连接问题':'现在开始'):'待完成',ready?'现在开始':'待 Tunnel 就绪'];const done=[configured,ready,false];states.forEach((el,i)=>{el.classList.toggle('done',done[i]);if(i===current)el.setAttribute('aria-current','step');else el.removeAttribute('aria-current');labels[i].textContent=status[i]})}
function updateTunnelClientGuidance(os){const mac=os==='darwin',other=os==='windows'||os==='linux';$('tunnelClientMacHint').classList.toggle('hidden',!mac);$('tunnelClientOtherHint').classList.toggle('hidden',!other);$('tunnelClientUnknownHint').classList.toggle('hidden',mac||other)}
async function refresh(){try{const s=await api('/api/status');updateSetupGuide(s);updateTunnelClientGuidance(s.os);const d=s.desktop_environment||{};let desktopNote=d.note||'无法读取桌面环境信息。';if(d.session_type)desktopNote=`当前会话：${d.session_type}。${desktopNote}`;if(d.input_ready===false)desktopNote+=' 未检测到可用的 X11 键鼠控制环境或 xdotool。';if(d.window_list_ready===false&&d.os==='Linux')desktopNote+=' 未检测到 wmctrl，窗口列表不可用。';if(d.screenshot_tool_available===false)desktopNote+=' 未检测到受支持的截图工具。';if(d.pointer_helper_installed===false)desktopNote+=' 未检测到 cliclick，macOS 鼠标控制不可用。';$('desktopPreflight').textContent=desktopNote;$('dot').classList.toggle('on',s.tunnel_running);$('statusText').textContent=s.tunnel_ready?'Tunnel 已就绪':s.tunnel_starting?'Tunnel 正在启动':s.tunnel_running?'Tunnel 客户端运行中':s.tunnel_configured?'等待连接':'需要配置';const tunnelBusy=!!s.tunnel_running||!!s.tunnel_starting;$('connectBtn').classList.toggle('hidden',tunnelBusy);$('stopBtn').classList.toggle('hidden',!tunnelBusy);$('deviceName').textContent=s.hostname||'本机';$('deviceMeta').textContent=`${s.os} · ${s.arch}`;$('osInfo').textContent=s.os;$('archInfo').textContent=s.arch;$('tunnelInfo').textContent=s.tunnel_ready?'已就绪':s.tunnel_starting?'正在启动':s.tunnel_running?'进程运行中':s.tunnel_configured?'已配置':'未配置';$('keyState').textContent=s.runtime_key_configured?'已保存':'未配置';$('clearRuntimeKeyButton').classList.toggle('hidden',!s.runtime_key_configured);$('clearRuntimeKeyButton').disabled=tunnelBusy;$('tunnelClientVersion').textContent=s.tunnel_client_version||'启动时检查';$('version').textContent='v'+s.version;$('lastCheck').textContent=s.last_successful_check&&s.last_successful_check!=='0001-01-01T00:00:00Z'?new Date(s.last_successful_check).toLocaleString():'尚无记录';const errorPanel=$('tunnelError');const started=Date.parse(s.started_at);const checked=Date.parse(s.last_successful_check);const waitingFrom=Math.max(Number.isFinite(started)?started:0,Number.isFinite(checked)&&checked>0?checked:0);const waitingForReady=!!s.tunnel_running&&!s.tunnel_ready&&waitingFrom>0&&Date.now()-waitingFrom>30000;errorPanel.classList.toggle('hidden',!s.last_error&&!waitingForReady);errorPanel.classList.toggle('waiting',!s.last_error&&waitingForReady);setTextIfChanged('tunnelErrorTitle',s.last_error?'连接未成功':'连接仍在等待');if(s.last_error){setTextIfChanged('tunnelErrorDetail',s.last_error);setTextIfChanged('tunnelErrorAdvice',recoveryAdvice(`${s.last_error}\n${$('logs').textContent||''}`))}else if(waitingForReady){setTextIfChanged('tunnelErrorDetail',checked>0?`Tunnel 上次就绪检查时间：${new Date(checked).toLocaleString()}；目前仍未恢复。`:'Tunnel 客户端已运行 30 秒，但尚未通过就绪检查。');setTextIfChanged('tunnelErrorAdvice',recoveryAdvice($('logs').textContent||''))}}catch(e){$('statusText').textContent='服务不可用'}}
async function loadLogs(){try{const r=await api('/api/logs');$('logs').textContent=(r.lines||[]).join('\n')||'暂无日志';$('logs').scrollTop=$('logs').scrollHeight}catch(e){}}
async function exportDiagnostics(){try{const report=await api('/api/diagnostics');const blob=new Blob([JSON.stringify(report,null,2)],{type:'application/json'});const link=document.createElement('a');link.href=URL.createObjectURL(blob);link.download=`Nexora-diagnostics-${new Date().toISOString().slice(0,10)}.json`;link.click();setTimeout(()=>URL.revokeObjectURL(link.href),1000);toast('诊断文件已导出；分享前请检查本机路径或其他标识信息')}catch(e){toast(e.message)}}
async function decideCommandApproval(id,approved,card){card.querySelectorAll('button').forEach(b=>b.disabled=true);try{await api('/api/command-approval',{id,approved});toast(approved?'已批准这一次命令':'已拒绝命令')}catch(e){toast(e.message.includes('expired')?'确认请求已过期，命令未执行':e.message)}finally{approvalSignature='';await loadCommandApprovals();const next=$('approvals').querySelector('button');(next&&!next.disabled?next:$('refreshButton')).focus()}}
async function loadCommandApprovals(){
  if(approvalPollRunning)return;
  approvalPollRunning=true;
  try{
    const r=await api('/api/command-approvals');
    const list=r.approvals||[];
    const signature=JSON.stringify(list);
    if(signature===approvalSignature)return;
    const currentIDs=new Set(list.map(item=>item.id));
    const newCount=list.reduce((count,item)=>count+(!approvalRequestIDs.has(item.id)?1:0),0);
    const removedCount=[...approvalRequestIDs].filter(id=>!currentIDs.has(id)).length;
    approvalRequestIDs=currentIDs;
    approvalSignature=signature;
    const panel=$('approvalPanel');
    const container=$('approvals');
    container.replaceChildren();
    $('approvalAnnouncement').textContent=newCount>0?`有 ${newCount} 条新命令等待确认；当前共 ${list.length} 条。请检查命令、工作目录和权限请求。`:removedCount>0?`有 ${removedCount} 条命令已不再等待本机确认；请在 ChatGPT 查看各任务状态。当前待确认 ${list.length} 条。`:list.length===0?'当前没有等待本机确认的命令。':`待确认命令列表已更新；当前共 ${list.length} 条。请检查命令、工作目录和权限请求。`;
    panel.classList.toggle('hidden',list.length===0);
    for(const item of list){
      const card=document.createElement('article');
      card.className='approval-item';
      const requestRef=item.id.slice(0,8);
      const head=document.createElement('header');
      const title=document.createElement('strong');
      title.id=`approval-title-${item.id}`;
      title.textContent=item.admin?`管理员命令请求 ${requestRef}`:`命令请求 ${requestRef}`;
      card.setAttribute('aria-labelledby',title.id);
      const expires=document.createElement('span');
      expires.textContent=`有效至 ${new Date(item.expires_at).toLocaleTimeString()}`;
      head.append(title,expires);
      const command=document.createElement('pre');
      command.id=`approval-command-${item.id}`;
      const visibleCommand=displayApprovalText(item.command);
      command.textContent=visibleCommand.text;
      command.setAttribute('aria-label','命令内容');
      card.append(head,command);
      let hiddenCharactersEscaped=visibleCommand.escaped;
      if(item.cwd){
        const cwd=document.createElement('p');
        cwd.className='approval-cwd';
        cwd.id=`approval-cwd-${item.id}`;
        const visibleCWD=displayApprovalText(item.cwd);
        cwd.textContent=`工作目录：${visibleCWD.text}`;
        hiddenCharactersEscaped=hiddenCharactersEscaped||visibleCWD.escaped;
        card.append(cwd);
      }
      if(hiddenCharactersEscaped){
        const note=document.createElement('p');
        note.className='approval-display-note';
        note.id=`approval-note-${item.id}`;
        note.textContent='命令或路径中的不可见控制字符已转义显示；批准后执行原始内容。';
        card.append(note);
      }
      if(item.admin){
        const warning=document.createElement('p');
        warning.className='approval-admin';
        warning.textContent='批准后还会出现操作系统管理员授权提示。拒绝系统提示将不会执行命令。';
        card.append(warning);
      }
      const actions=document.createElement('div');
      actions.className='approval-actions';
      const deny=document.createElement('button');
      deny.className='btn danger';
      deny.textContent='拒绝';
      deny.setAttribute('aria-label',`拒绝命令请求 ${requestRef}`);
      deny.addEventListener('click',()=>decideCommandApproval(item.id,false,card));
      const allow=document.createElement('button');
      allow.className='btn primary';
      allow.textContent='仅允许这一次';
      allow.setAttribute('aria-label',`仅允许命令请求 ${requestRef} 一次`);
      allow.addEventListener('click',()=>decideCommandApproval(item.id,true,card));
      actions.append(deny,allow);
      card.append(actions);
      container.append(card);
    }
  }catch(e){}finally{
    approvalPollRunning=false;
  }
}
function toggleSecret(){const f=$('apiKey'),button=$('toggleSecretButton'),show=f.type==='password';f.type=show?'text':'password';button.textContent=show?'隐藏':'显示';button.setAttribute('aria-pressed',String(show));button.setAttribute('aria-label',show?'隐藏运行时 API Key':'显示运行时 API Key')}
async function clearRuntimeKey(){if(!window.confirm('清除本机保存的 Tunnel 运行时 API Key？此操作不会撤销 OpenAI Platform 上的密钥；如需彻底失效，请同时在 Platform 撤销该 Key。'))return;const button=$('clearRuntimeKeyButton');button.disabled=true;try{await api('/api/runtime-key/clear',{});$('apiKey').value='';toast('已清除本机保存的 Key；如需彻底失效，请在 OpenAI Platform 撤销它');await refresh()}catch(e){toast(e.message);await refresh()}finally{button.disabled=false}}
for(const id of ['tunnelId','apiKey','clientPath','fileRoots'])$(id).addEventListener('input',()=>markFormDirty(id));for(const id of ['allowCommands','allowFiles','allowDesktop','allowBrowser','allowAdmin','startOnLaunch'])$(id).addEventListener('change',()=>markFormDirty(id));window.addEventListener('beforeunload',event=>{if(formRevision!==savedRevision){event.preventDefault();event.returnValue=''}});
setupTabs();configLoadPromise=loadConfig();configLoadPromise.then(refresh);loadLogs();loadCommandApprovals();setInterval(()=>{refresh();loadLogs()},5000);setInterval(loadCommandApprovals,1200);
$('refreshButton').addEventListener('click',refresh);$('saveConfigButton').addEventListener('click',()=>saveConfig());$('saveChangesButton').addEventListener('click',()=>saveConfig());$('connectBtn').addEventListener('click',connect);$('stopBtn').addEventListener('click',stopTunnel);$('quitAppButton').addEventListener('click',quitApp);$('toggleSecretButton').addEventListener('click',toggleSecret);$('clearRuntimeKeyButton').addEventListener('click',clearRuntimeKey);$('selectFileRootButton').addEventListener('click',selectFileRoot);$('selectTunnelClientButton').addEventListener('click',selectTunnelClient);$('refreshLogsButton').addEventListener('click',loadLogs);$('exportDiagnosticsButton').addEventListener('click',exportDiagnostics);
