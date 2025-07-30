

function auth(){
    if (!exist_profile() ) {
        next = "login"
        return
    }
}