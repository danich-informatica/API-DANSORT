import{_ as O}from"./C7a6sH2f.js";import{_ as C}from"./BAJOmgxM.js";import{F as s,_ as d,v as _}from"./C1XiG2KJ.js";import{Z as P,e as q,r as y,k as m,g as U,c as u,a as F,x as L,b as p,t as T,j as e,C as A,a7 as R,o as S}from"#entry";import{g as b}from"./9e-Yg0Sx.js";const g={name:{rules:["required"],type:s.TEXT},description:{rules:["required"],type:s.TEXT},file:{rules:["fileRequired"],type:s.FILE}},G={name:{rules:["required"],type:s.TEXT},description:{rules:["required"],type:s.TEXT}};var i=(t=>(t.ADD_ETIQUETTE_TEMPLATE="AddEtiquetteTemplate",t.UPDATE_ETIQUETTE_TEMPLATE="UpdateEtiquetteTemplate",t.TOGGLE_ETIQUETTE_TEMPLATE="ToggleEtiquetteTemplate",t.SHOW_ETIQUETTE_PREVIEW="ShowEtiquettePreview",t))(i||{});const v=[{name:i.ADD_ETIQUETTE_TEMPLATE,show:!1},{name:i.UPDATE_ETIQUETTE_TEMPLATE,show:!1},{name:i.TOGGLE_ETIQUETTE_TEMPLATE,show:!1},{name:i.SHOW_ETIQUETTE_PREVIEW,show:!1}],M=()=>({etiquette_templates:[{description:"PROCESO 203030",file:null,name:"TEMPLATE 203030",template:`^XA
            ^CI28
            ^FX Título
            ^FT355,20^A0R,35,35^FDCerezas / Cherries^FS 
            ^FT355,350^A0R,30,30^FDSANTINA^FS
            ^FX Línea horizontal debajo del título
            ^FO345,0^GB2,1000,2^FS
            
            ^FX Columna 1
            ^FT310,20^A0R,26,20^FDGROWER / PRODUCTOR^FS
            ^FT270,30^A0R,20,20^FDC S G: 165240^FS
            ^FT250,30^A0R,20,20^FDG G N: 4063061567222^FS
            ^FX Subcolumnas dentro de la Columna 1
            ^FT220,30^A0R,15,15^FDSAGRADA FAMILIA^FS
            ^FT205,30^A0R,15,15^FDCURICO^FS
            ^FT190,30^A0R,15,15^FD7 REGIÓN^FS
            ^FT175,30^A0R,16,16^FDCHILE^FS
        
            ^FT30,90^A0R,23,23^FD150000009486^FS
            ^FO55,85^BXN,2,6^FDQA,150000009486^FS
            
            ^FX Separador entre columna 1 y 2
            ^FO10,235^GB325,2,2^FS

            ^FX Columna 2
            ^FT310,250^A0R,26,20^FDPACKING / PACKHOUSE^FS
            ^FT270,260^A0R,16,16^FDHUAQUEN EXPORT SERVICIOS SPA^FS
            ^FT245,260^A0R,20,20^FDCSP: 3129652^FS
            ^FT220,260^A0R,15,15^FDROMERAL^FS
            ^FT205,260^A0R,15,15^FDCURICÓ^FS
            ^FT190,260^A0R,15,15^FD7 REGIÓN^FS
            ^FT175,260^A0R,15,15^FDCHILE^FS
            
            ^FT95,260^A0R,28,20^FDNET WEIGHT/PESO NETO:^FS
            ^FT95,480^A0R,30,20^FD2.5 KG^FS
            ^FT65,260^A0R,20,20^FDCAT1^FS
            ^FT65,360^A0R,20,20^FDSELLADORA: 6^FS
            ^FT35,260^A0R,20,20^FDREPUBLIC OF CHILE^FS

            ^FX Separador entre columna 2 y 3
            ^FO10,545^GB325,2,2^FS

            ^FX Columna 3
            ^FT310,560^A0R,26,20^FDWHEN PACKED^FS
            ^FT270,570^A0R,20,15^FDDATE / FECHA:^FS
            ^FT270,700^A0R,20,20^FD14/11/24^FS
            ^FT245,570^A0R,20,20^FD16^FS
            ^FT225,570^A0R,20,18^FDTURNO:^FS
            ^FT225,700^A0R,20,18^FD1^FS
            ^FT205,570^A0R,20,18^FDEXIT / SALIDA:^FS
            ^FT205,710^A0R,20,18^FD14^FS
            ^FT185,570^A0R,20,20^FDHORA:^FS
            ^FT185,700^A0R,20,20^FD13:00:51^FS
            ^FT165,570^A0R,20,20^FDLOTE:^FS
            ^FT165,700^A0R,20,20^FD15^FS
            
            ^FO55,600^GB80,150,50,B^FS
            ^FR^FT70,640^A0R,60,50^FDSAG^FS^FR
            
            ^FT30,615^A0R,22,30^FD1HR25FB^FS

            ^FT0,50^A0R,23,18^FDREPUBLIC OF CHILE^FS
            ^FT0,245^A0R,23,18^FDEXPORTED BY: HUAQUEN EXPORT SPA^FS
            ^FT0,630^A0R,23,18^FDCSE: 3104449^FS
        ^XZ`}],modals:v}),N={async getEtiquetteTemplates(){},async getZplPreview(t){try{const a=await b(`https://api.labelary.com/v1/printers/8dpmn/labels/4x6/0/${t}`);if(a&&a.data)return URL.createObjectURL(new Blob([a.data],{type:"image/png"}));throw new Error("No data received from Labelary API")}catch(a){console.error(a)}}},V={async addEtiquetteTemplate(t){}},X={async updateEtiquetteTemplate(t){}},w={async deleteEtiquetteTemplate(t){}},x={...N,...V,...X,...w},B={},c=P("myEtiquetteTemplateStore",{state:M,actions:x,getters:B}),H=()=>{const t=c();return{addEtiquetteTemplate:async a=>await t.addEtiquetteTemplate(a)}},h=()=>{const t=c();return{updateEtiquetteTemplate:async a=>await t.updateEtiquetteTemplate(a)}},Q=()=>{const{addEtiquetteTemplate:t}=H();return{title:"INGRESO DE TEMPLATE DE ETIQUETA",button_title:"INGRESAR TEMPLATE",fn:a=>t(a)}},W=()=>{const{updateEtiquetteTemplate:t}=h();return{title:"EDICIÓN DE TEMPLATE",button_title:"GUARDAR CAMBIOS",fn:a=>()=>{t(a)}}},k={class:"flex flex-col justify-center items-center text-dgreen2 text-3"},j={class:"title-4 text-dgreen1"},K={key:0,class:"pt-2 flex flex-col items-center mt-5"},$={class:"text-5 text-center text-dred2"},z={class:"flex flex-col items-center mt-5"},Z={class:"text-5 text-center text-dred2"},Y={class:"flex flex-col items-center mt-5"},J={class:"text-5 text-center text-dred2"},ne=q({__name:"EtiquetteTemplateForm",props:{template:{}},setup(t){const a=t,o=y({name:"",description:"",file:null}),D=m(()=>a.template?G:g),E=m(()=>a.template?W():Q());return U(()=>{a.template&&(o.value={...a.template,file:a.template.file??null})}),(l,n)=>{const I=O,f=C;return S(),u("div",k,[F("h1",j,T(e(E).title),1),t.template?L("",!0):(S(),u("div",K,[p(I,{modelValue:e(o).file,"onUpdate:modelValue":n[0]||(n[0]=r=>e(o).file=r),label:"INGRESAR ARCHIVO DE TEMPLATE",accept:".zpl"},null,8,["modelValue"]),F("label",$,T(("validateFieldOnForm"in l?l.validateFieldOnForm:e(d))(e(o).file,["fileRequired"],("FormValidatorTypes"in l?l.FormValidatorTypes:e(s)).FILE)),1)])),F("div",z,[n[4]||(n[4]=F("label",null,"INGRESAR NOMBRE",-1)),A(F("input",{"onUpdate:modelValue":n[1]||(n[1]=r=>e(o).name=r),class:"border border-dgreen1 text-5 h-8 w-52 rounded-md p-1",type:"text"},null,512),[[R,e(o).name]]),F("label",Z,T(("validateFieldOnForm"in l?l.validateFieldOnForm:e(d))(e(o).name,["required"],("FormValidatorTypes"in l?l.FormValidatorTypes:e(s)).TEXT)),1)]),F("div",Y,[n[5]||(n[5]=F("label",null,"INGRESAR DESCRIPCIÓN",-1)),A(F("input",{"onUpdate:modelValue":n[2]||(n[2]=r=>e(o).description=r),class:"border border-dgreen1 text-5 h-8 w-52 rounded-md p-1",type:"text"},null,512),[[R,e(o).description]]),F("label",J,T(("validateFieldOnForm"in l?l.validateFieldOnForm:e(d))(e(o).description,["required"],("FormValidatorTypes"in l?l.FormValidatorTypes:e(s)).TEXT)),1)]),p(f,{onClick:n[3]||(n[3]=r=>e(E).fn(e(o))),is_tooltip_visible:!1,button_text:e(E).button_title,button_icon_name:null,button_class:"w-52 bg-dgreen2 text-white mt-5 rounded-md",is_disabled:!("validateForm"in l?l.validateForm:e(_))(e(o),e(D))},null,8,["button_text","is_disabled"])])}}});export{i as M,ne as _,c as u};
