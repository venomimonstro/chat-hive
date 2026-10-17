export function scopedChatMessages<T extends {chat_id:string}>(items:T[],chatID?:string):T[]{return chatID?items.filter(x=>x.chat_id===chatID):[];}
export function canCacheChatMessages<T extends {chat_id:string}>(items:T[],chatID:string):boolean{return items.every(x=>x.chat_id===chatID);}
