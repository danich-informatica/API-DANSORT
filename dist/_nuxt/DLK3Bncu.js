import{e as T,r as e,g as s,c as D,a as o,j as c,F as i,o as u}from"#entry";const E={class:"rotate-270 w-full max-w-2xl mx-auto p-4 border rounded-lg shadow-md flex justify-center items-center"},d=["src"],p=T({__name:"EtiquetteTemplatePreview",setup(C){const r=e(`^XA
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
        ^XZ`),F=e(null),l=e(!1),R=e(null);async function A(){l.value=!0,R.value=null,F.value&&(URL.revokeObjectURL(F.value),F.value=null);try{const n=`https://api.labelary.com/v1/printers/8dpmm/labels/2x4/0/${r.value}`,S=await $fetch(n,{method:"GET",headers:{Accept:"image/png"}});F.value=URL.createObjectURL(S)}catch(a){console.error("Error al generar la etiqueta:",a),R.value="Error de CORS: La petición fue bloqueada por el navegador."}finally{l.value=!1}}return s(async()=>{await A()}),(a,t)=>(u(),D(i,null,[t[0]||(t[0]=o("h1",{class:"title-2 text-dgreen1 text-center font-bold mb-2"},"Previsualizador de Etiquetas ZPL",-1)),o("div",E,[o("img",{src:c(F)??"",alt:"Previsualización de la etiqueta",class:"object-contain mx-auto"},null,8,d)])],64))}});export{p as _};
