package license

// Consent is the exact text shown before registering, for ConsentVersion. Changing any word here means a new
// ConsentVersion (agreed with the codeDB backend, which records the version the user saw).
// lens-2026-1: approved by codeDB legal on 2026-10-22 (same text as the lens-2026-0 draft); en is a faithful
// translation of pt-BR.
// lens-2026-2: approved by codeDB legal on 2026-11-06 (pt-BR and en); adds the country (legitimate interest, art. 7 IX).
type ConsentText struct {
	PrivacyAck string // required checkbox, unchecked by default
	Marketing  string // optional checkbox, unchecked by default, separate from the first
	Notice     string // short text below the checkboxes
	Sent       string // after a successful registration
}

var Consent = map[string]ConsentText{
	"pt-BR": {
		PrivacyAck: "Li e concordo com a Política de Privacidade",
		Marketing:  "Quero receber novidades e contato comercial do codeDB por e-mail",
		Notice: "Usamos seu nome e e-mail para emitir e enviar sua chave grátis. Também pedimos seu país, para entendermos onde o Lens é usado e priorizarmos idiomas e melhorias; ele não é usado para contato comercial sem a sua confirmação. " +
			"Empresa, cargo e número de instâncias são opcionais. Nenhum dado dos seus bancos sai deste computador. " +
			"O contato comercial só acontece se você marcar a segunda opção e confirmar pelo e-mail. Você pode se opor ao uso do país, se descadastrar " +
			"ou apagar seus dados a qualquer momento pelos links do e-mail.",
		Sent: "Enviamos uma cópia da chave para seu e-mail.",
	},
	"en": {
		PrivacyAck: "I have read and agree to the Privacy Policy",
		Marketing:  "I want to receive news and sales contact from codeDB by e-mail",
		Notice: "We use your name and e-mail to issue and send your free key. We also ask for your country, to understand where Lens is used and to prioritize languages and improvements; it is not used for sales contact unless you confirm. " +
			"Company, role and number of instances are optional. No data from your databases leaves this computer. " +
			"Sales contact only happens if you tick the second option and confirm by e-mail. You can object to the use of your country, unsubscribe " +
			"or delete your data at any time through the links in the e-mail.",
		Sent: "We sent a copy of the key to your e-mail.",
	},
}
