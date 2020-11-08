{pkgs ? import <nixpkgs> {}, commit ? ""}:                                                                           

{
	ci-shell=pkgs.mkShell {                                                                                             
  buildInputs = [                                                                                     
        python38                                                                                      
        protobuf3_13                                                                                  
        grpc                                                                                          
        git                                                                                           
  ];                                                                                                  
  shellHook= ''                                                                                       
  ./convert.sh                                                                                          '';                                                                                                 
  };

   dev-shell=pkgs.mkShell {                                                                                             
   buildInputs = [                                                                                     
        python38                                                                                      
        protobuf3_13                                                                                  
        grpc                                                                                          
        git                                                                                           
  ];                                                                                                  
  shellHook= ''                                                                                       
  ./convert.sh                                                                                          '';                                                                                                 
};                                                                                                    
                                                                                                     

}

